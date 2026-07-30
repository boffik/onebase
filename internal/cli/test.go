package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/ui"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Прогнать тесты уровня конфигурации (обработки kind: test)",
	Long: `Находит тест-обработки (в YAML указан kind: test), выполняет процедуру
Выполнить() каждой офлайн — как procrun — и собирает результаты встроенных
проверок Утверждать.*. Печатает по проверке на строку и итог; при провале
любой проверки или ошибке выполнения завершается с ненулевым кодом — пригодно
для pre-commit/CI.

Внутри теста доступен объект Утверждать:
  Утверждать.Равно(Факт, Ожидание, "описание");
  Утверждать.НеРавно(Факт, Ожидание, "описание");
  Утверждать.Истина(Условие, "описание");
  Утверждать.Ложь(Условие, "описание");
  Утверждать.Заполнено(Значение, "описание");
  Утверждать.Провалить("описание");

Примеры:
  onebase test --project . --sqlite prodbase.db
  onebase test --project . --run Телефон`,
	RunE:          runTest,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	addBaseFlags(testCmd)
	testCmd.Flags().String("run", "", "маска по имени теста (регистронезависимая подстрока)")
	testCmd.Flags().String("isolation", "transaction",
		"изоляция данных между тестами: transaction (откат после каждого) | none")
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, _ []string) error {
	bc, err := resolveBase(cmd)
	if err != nil {
		return err
	}
	defer bc.Cleanup()

	ctx := context.Background()
	db, err := bc.OpenDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	proj, err := project.Load(bc.Dir)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	defer proj.Close()

	// Тесты гоняются на готовой схеме: применяем миграции (идемпотентно) —
	// иначе на свежей/`:memory:` базе запись справочников/документов падает на
	// «no such table». Плюс схема аудита, как procrun/run/dev.
	if err := applyAllMigrations(ctx, db, proj); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := db.EnsureAuditSchema(ctx); err != nil {
		return fmt.Errorf("audit schema: %w", err)
	}

	filter, _ := cmd.Flags().GetString("run")
	isolation, _ := cmd.Flags().GetString("isolation")
	switch isolation {
	case "", ui.IsolationTransaction, ui.IsolationNone:
	default:
		return fmt.Errorf("неизвестный режим --isolation %q (доступны transaction, none)", isolation)
	}
	res, err := ui.RunTests(ctx, proj, db, ui.TestRunOptions{Filter: filter, Isolation: isolation})
	if err != nil {
		return err
	}

	printTestResults(res)

	if len(res.Cases) == 0 {
		// Нет тестов — не провал (конфигурация может их ещё не завести), но
		// сообщаем явно, чтобы «зелёный» прогон не вводил в заблуждение.
		fmt.Fprintln(os.Stdout, "Тесты не найдены (обработки с kind: test).")
		return nil
	}

	if !res.OK() {
		bc.Cleanup() // defer не выполнится при os.Exit
		db.Close()
		os.Exit(1)
	}
	return nil
}

func printTestResults(res ui.TestRunResult) {
	for _, c := range res.Cases {
		fmt.Fprintf(os.Stdout, "▶ %s\n", c.Name)
		for _, o := range c.Asserts {
			if o.Passed {
				fmt.Fprintf(os.Stdout, "  ok    — %s\n", o.Desc)
			} else if o.Detail != "" {
				fmt.Fprintf(os.Stdout, "  ПРОВАЛ — %s (%s)\n", o.Desc, o.Detail)
			} else {
				fmt.Fprintf(os.Stdout, "  ПРОВАЛ — %s\n", o.Desc)
			}
		}
		if c.Err != nil {
			fmt.Fprintf(os.Stdout, "  ОШИБКА — %s\n", c.Err.Error())
		}
		if len(c.Asserts) == 0 && c.Err == nil {
			fmt.Fprintln(os.Stdout, "  (без единой проверки — тест считается неуспешным)")
		}
		fmt.Fprintf(os.Stdout, "  %s  (%s)\n", caseSummary(c), fmtDuration(c.Duration))
	}

	tests, passedTests, asserts, failedAsserts := res.Totals()
	fmt.Fprintln(os.Stdout, "── Итог ──")
	fmt.Fprintf(os.Stdout, "Тестов: %d, успешно: %d, провалено: %d\n",
		tests, passedTests, tests-passedTests)
	fmt.Fprintf(os.Stdout, "Проверок: %d, провалено: %d\n", asserts, failedAsserts)
}

func caseSummary(c ui.TestCaseResult) string {
	total := c.Passed + c.Failed
	if c.OK() {
		return fmt.Sprintf("OK: %d проверок", total)
	}
	if c.Err != nil {
		return fmt.Sprintf("ОШИБКА: %d/%d проверок прошло", c.Passed, total)
	}
	return fmt.Sprintf("ПРОВАЛ: %d из %d проверок", c.Failed, total)
}

func fmtDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
