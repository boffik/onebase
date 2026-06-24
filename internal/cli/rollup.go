package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
	"github.com/spf13/cobra"
)

var rollupCmd = &cobra.Command{
	Use:   "rollup",
	Short: "Свёртка базы: свернуть остатки регистров на дату",
	Long: `Свёртка информационной базы (план 74). На выбранную дату остатки регистров
накопления сворачиваются в опорные записи (синтетический регистратор), а старые
движения удаляются. Документы до даты по умолчанию удаляются (--keep-documents
оставляет их без проведения). Выставляется дата запрета проведения.

ОПЕРАЦИЯ НЕОБРАТИМА — сделайте резервную копию (onebase backup) перед запуском.

Примеры:
  onebase rollup --project ./trade --sqlite trade.db --date 2025-01-01 --dry-run
  onebase rollup --id <baseID> --date 2025-01-01 --keep-documents`,
	RunE: runRollup,
}

func init() {
	addBaseFlags(rollupCmd)
	rollupCmd.Flags().String("date", "", "дата свёртки ГГГГ-ММ-ДД (обязательно)")
	rollupCmd.Flags().String("registers", "", "регистры накопления через запятую (по умолчанию — все)")
	rollupCmd.Flags().Bool("dry-run", false, "только показать, что будет сделано")
	rollupCmd.Flags().Bool("keep-documents", false, "не удалять документы до даты, а снять с них проведение")
	rollupCmd.Flags().Bool("yes", false, "не запрашивать подтверждение")
	rootCmd.AddCommand(rollupCmd)
}

func runRollup(cmd *cobra.Command, _ []string) error {
	dateStr, _ := cmd.Flags().GetString("date")
	if dateStr == "" {
		return fmt.Errorf("укажите --date ГГГГ-ММ-ДД")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("неверная дата %q (ожидается ГГГГ-ММ-ДД): %w", dateStr, err)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	keepDocs, _ := cmd.Flags().GetBool("keep-documents")
	assumeYes, _ := cmd.Flags().GetBool("yes")
	regsArg, _ := cmd.Flags().GetString("registers")

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

	regNames := selectedRegisterNames(proj.Registers, regsArg)
	if len(regNames) == 0 {
		return fmt.Errorf("нет регистров накопления для свёртки")
	}

	opts := storage.RollupOptions{
		Date:            date,
		Registers:       regNames,
		DeleteDocuments: !keepDocs,
	}

	prev, err := db.RollupPreview(ctx, proj.Registers, proj.Entities, opts)
	if err != nil {
		return fmt.Errorf("предпросмотр свёртки: %w", err)
	}
	fmt.Fprintln(os.Stdout, "Предпросмотр свёртки:")
	printRollupReport(os.Stdout, prev, keepDocs)

	if dryRun {
		fmt.Fprintln(os.Stdout, "\n(--dry-run: изменения не внесены)")
		return nil
	}

	if !assumeYes {
		fmt.Fprintf(os.Stdout,
			"\nВНИМАНИЕ: операция необратима. Сделайте резервную копию (onebase backup).\nПродолжить свёртку на %s? [y/N]: ",
			date.Format("02.01.2006"))
		var ans string
		fmt.Scanln(&ans)
		if strings.ToLower(strings.TrimSpace(ans)) != "y" {
			fmt.Fprintln(os.Stdout, "Отменено.")
			return nil
		}
	}

	rep, err := db.Rollup(ctx, proj.Registers, proj.Entities, opts)
	if err != nil {
		return fmt.Errorf("свёртка: %w", err)
	}
	fmt.Fprintln(os.Stdout, "\nГотово:")
	printRollupReport(os.Stdout, rep, keepDocs)
	return nil
}

// selectedRegisterNames — имена регистров для свёртки: из --registers (через
// запятую) или все регистры накопления конфигурации.
func selectedRegisterNames(all []*metadata.Register, arg string) []string {
	if strings.TrimSpace(arg) == "" {
		names := make([]string, 0, len(all))
		for _, r := range all {
			if r.IsTurnover() {
				continue // оборотные регистры не сворачиваются
			}
			names = append(names, r.Name)
		}
		return names
	}
	var out []string
	for _, n := range strings.Split(arg, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func printRollupReport(w io.Writer, rep storage.RollupReport, keepDocs bool) {
	fmt.Fprintf(w, "  Дата свёртки: %s\n", rep.Cutoff.Format("02.01.2006"))
	for _, r := range rep.Registers {
		fmt.Fprintf(w, "  %-32s движений: %6d  опорных остатков: %5d\n",
			r.Name, r.FoldedMovements, r.OpeningRows)
	}
	if keepDocs {
		fmt.Fprintln(w, "  Документы: снять проведение (не удалять)")
	} else {
		fmt.Fprintf(w, "  Документы к удалению: %d\n", rep.DeletedDocs)
		if rep.DanglingRefs > 0 {
			fmt.Fprintf(w, "  ⚠ повиснет ссылок на удаляемые документы: %d\n", rep.DanglingRefs)
		}
	}
}
