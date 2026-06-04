package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ivantit66/onebase/internal/backup"
	"github.com/ivantit66/onebase/internal/storage"
)

var (
	backupDB  string
	backupOut string
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a gzipped SQL backup of the database",
	Example: "  onebase backup --db postgres://localhost/mydb --out ./backups/",
	RunE:  runBackup,
}

func init() {
	backupCmd.Flags().StringVar(&backupDB, "db", "", "PostgreSQL connection string (required)")
	backupCmd.Flags().StringVar(&backupOut, "out", ".", "output directory for the backup file")
	_ = backupCmd.MarkFlagRequired("db")
}

func runBackup(cmd *cobra.Command, args []string) error {
	outDir, err := filepath.Abs(backupOut)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Создание бэкапа в %s ...\n", outDir)
	path, err := backup.Dump(cmd.Context(), backupDB, outDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Бэкап сохранён: %s\n", path)
	return nil
}

var (
	restoreDB   string
	restoreFile string
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore database from a backup file",
	Example: "  onebase restore --db postgres://localhost/mydb --file ./backups/backup_mydb_2026-05-06_14-30.sql.gz",
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().StringVar(&restoreDB, "db", "", "PostgreSQL connection string (required)")
	restoreCmd.Flags().StringVar(&restoreFile, "file", "", "path to the .sql.gz backup file (required)")
	_ = restoreCmd.MarkFlagRequired("db")
	_ = restoreCmd.MarkFlagRequired("file")
}

func runRestore(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(os.Stdout, "Восстановление из %s ...\n", restoreFile)
	if err := backup.Restore(cmd.Context(), restoreDB, restoreFile); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Восстановление завершено.")
	return nil
}

var (
	demoResetDB   string
	demoResetFile string
)

var demoResetCmd = &cobra.Command{
	Use:   "demo-reset",
	Short: "Restore demo business data from a .obz backup (keeps users, roles and sessions)",
	Long: "Восстанавливает бизнес-данные из .obz, сохраняя таблицы авторизации " +
		"(_users, _sessions, _roles, _user_roles). Та же операция, что выполняет " +
		"регламентное задание DemoReset по расписанию — но запускается немедленно. " +
		"Удобно дёргать из деплой-скрипта после заливки свежего .obz.",
	Example: "  onebase demo-reset --db postgres://localhost/mydb --file ./demo.obz",
	RunE:    runDemoReset,
}

func init() {
	demoResetCmd.Flags().StringVar(&demoResetDB, "db", "", "PostgreSQL connection string (required)")
	demoResetCmd.Flags().StringVar(&demoResetFile, "file", "", "path to the .obz backup file (required)")
	_ = demoResetCmd.MarkFlagRequired("db")
	_ = demoResetCmd.MarkFlagRequired("file")
}

func runDemoReset(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	db, err := storage.Connect(ctx, demoResetDB)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Fprintf(os.Stdout, "Сброс демо-данных из %s ...\n", demoResetFile)
	report, err := backup.DemoReset(ctx, db, demoResetFile)
	if err != nil {
		return err
	}
	rows := 0
	for _, n := range report.Tables {
		rows += n
	}
	fmt.Fprintf(os.Stdout, "Готово: таблиц %d, строк %d.\n", len(report.Tables), rows)
	return nil
}
