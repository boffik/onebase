package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/ivantit66/onebase/internal/backup"
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
