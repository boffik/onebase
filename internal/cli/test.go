package cli

import (
	"context"
	"fmt"
	"os"

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

Внутри теста доступны:
  Утверждать.Равно/НеРавно/Истина/Ложь/Заполнено/Провалить(…, "описание");
  Часы.Установить(Дата)/Сбросить()   — заморозка ТекущаяДата()/ТекущаяДатаВремя();
  Мок.Email/Http/ОС/ИИ               — рекордеры внешних эффектов (почта/сеть/
                                        команды/ИИ не уходят наружу).

По умолчанию каждый тест идёт в своей транзакции с откатом (--isolation) — тесты
не оставляют данных. Формат вывода --format pretty|tap|junit (tap/junit — для CI).

Примеры:
  onebase test --project . --sqlite prodbase.db
  onebase test --project . --run Телефон
  onebase test --project . --sqlite :memory: --format junit --out report.xml`,
	RunE:          runTest,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	addBaseFlags(testCmd)
	testCmd.Flags().String("run", "", "маска по имени теста (регистронезависимая подстрока)")
	testCmd.Flags().String("isolation", "transaction",
		"изоляция данных между тестами: transaction (откат после каждого) | none")
	testCmd.Flags().String("format", "pretty", "формат отчёта: pretty | tap | junit")
	testCmd.Flags().String("out", "", "файл для отчёта (по умолчанию stdout)")
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, _ []string) error {
	isolation, _ := cmd.Flags().GetString("isolation")
	switch isolation {
	case "", ui.IsolationTransaction, ui.IsolationNone:
	default:
		return fmt.Errorf("неизвестный режим --isolation %q (доступны transaction, none)", isolation)
	}
	format, _ := cmd.Flags().GetString("format")
	switch format {
	case "", ui.FormatPretty, ui.FormatTAP, ui.FormatJUnit:
	default:
		return fmt.Errorf("неизвестный формат --format %q (доступны pretty, tap, junit)", format)
	}

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
	res, err := ui.RunTests(ctx, proj, db, ui.TestRunOptions{Filter: filter, Isolation: isolation})
	if err != nil {
		return err
	}

	if err := emitReport(cmd, res, format); err != nil {
		return err
	}

	if len(res.Cases) == 0 {
		// Нет тестов — не провал (конфигурация может их ещё не завести), но
		// сообщаем явно, чтобы «зелёный» прогон не вводил в заблуждение.
		fmt.Fprintln(os.Stderr, "Тесты не найдены (обработки с kind: test).")
		return nil
	}

	if !res.OK() {
		bc.Cleanup() // defer не выполнится при os.Exit
		db.Close()
		os.Exit(1)
	}
	return nil
}

// emitReport пишет отчёт в файл (--out) или в stdout. При записи в файл в
// stderr печатается краткая сводка, чтобы прогон не выглядел «немым».
func emitReport(cmd *cobra.Command, res ui.TestRunResult, format string) error {
	outPath, _ := cmd.Flags().GetString("out")
	if outPath == "" {
		return ui.WriteReport(os.Stdout, res, format)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("создать файл отчёта %q: %w", outPath, err)
	}
	defer f.Close()
	if err := ui.WriteReport(f, res, format); err != nil {
		return err
	}
	tests, passed, _, _ := res.Totals()
	fmt.Fprintf(os.Stderr, "Отчёт (%s) записан в %s — тестов: %d, успешно: %d\n",
		format, outPath, tests, passed)
	return nil
}
