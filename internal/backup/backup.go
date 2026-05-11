package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Dump creates a gzipped plain-SQL backup of the database at connStr.
// It writes the file to outDir and returns the full path of the created file.
// Requires pg_dump in PATH.
func Dump(ctx context.Context, connStr, outDir string) (string, error) {
	pgDump, err := findPgTool("pg_dump")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}

	dbName := dbNameFromDSN(connStr)
	ts := time.Now().Format("2006-01-02_15-04")
	filename := fmt.Sprintf("backup_%s_%s.sql.gz", dbName, ts)
	outPath := filepath.Join(outDir, filename)

	// pg_dump → stdout → gzip → file
	cmd := exec.CommandContext(ctx, pgDump, "--format=plain", "--no-owner", "--no-acl", connStr)
	r, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("pg_dump: %w", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		_ = cmd.Process.Kill()
		return "", err
	}

	gz := gzip.NewWriter(f)
	if _, err := io.Copy(gz, r); err != nil {
		f.Close()
		return "", err
	}
	if err := gz.Close(); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	if err := cmd.Wait(); err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("pg_dump завершился с ошибкой: %w", err)
	}
	return outPath, nil
}

// Restore restores a gzipped plain-SQL backup created by Dump into the database.
// Requires psql in PATH.
func Restore(ctx context.Context, connStr, filePath string) error {
	psql, err := findPgTool("psql")
	if err != nil {
		return err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("не удалось прочитать gzip-архив: %w", err)
	}
	defer gz.Close()

	cmd := exec.CommandContext(ctx, psql, "--no-password", connStr)
	cmd.Stdin = gz
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql завершился с ошибкой: %w", err)
	}
	return nil
}

// dbNameFromDSN extracts the database name from a connection string.
// Supports both URL (postgres://host/dbname) and DSN (dbname=foo) formats.
func dbNameFromDSN(connStr string) string {
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		if u, err := url.Parse(connStr); err == nil {
			name := strings.TrimPrefix(u.Path, "/")
			if name != "" {
				return sanitize(name)
			}
		}
	}
	// DSN key=value format
	for _, part := range strings.Fields(connStr) {
		if strings.HasPrefix(part, "dbname=") {
			return sanitize(strings.TrimPrefix(part, "dbname="))
		}
	}
	return "db"
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// findPgTool locates a PostgreSQL tool (pg_dump, psql) by first checking PATH,
// then searching common Windows install directories.
func findPgTool(name string) (string, error) {
	// Try PATH first
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		// Search standard PostgreSQL install dirs on Windows
		pgDirs := []string{
			`C:\Program Files\PostgreSQL`,
			`C:\Program Files (x86)\PostgreSQL`,
		}
		for _, base := range pgDirs {
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			// Iterate version dirs in reverse (newest first)
			for i := len(entries) - 1; i >= 0; i-- {
				binPath := filepath.Join(base, entries[i].Name(), "bin", name+".exe")
				if _, err := os.Stat(binPath); err == nil {
					return binPath, nil
				}
			}
		}
	}
	return "", fmt.Errorf("%s не найден; установите PostgreSQL и добавьте в PATH", name)
}
