package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil && runErr == nil {
		runErr = err
	}
	_ = r.Close()
	return buf.String(), runErr
}

func TestInstallWindowsServicePrintUsesSQLite(t *testing.T) {
	out, err := captureStdout(t, func() error {
		return installWindowsService(
			`C:\onebase\bin\onebase.exe`,
			"onebase-docflow",
			"docflow",
			"",
			`C:\onebase\data\docflow.db`,
			"sqlite",
			"file",
			`C:\onebase\project`,
			8080,
			true,
			true,
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `--sqlite "C:\onebase\data\docflow.db"`) {
		t.Fatalf("windows service command must use --sqlite, got:\n%s", out)
	}
	if strings.Contains(out, `--db ""`) {
		t.Fatalf("windows service command must not include empty --db, got:\n%s", out)
	}
	if !strings.Contains(out, `--project "C:\onebase\project"`) || !strings.Contains(out, "--watch") {
		t.Fatalf("windows service command lost project/watch args:\n%s", out)
	}
}

func TestInstallSystemdPrintUsesSQLite(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("user", "svc", "")
	out, err := captureStdout(t, func() error {
		return installSystemd(
			"/opt/onebase/onebase",
			"onebase-docflow",
			"docflow",
			"",
			"/var/lib/onebase/docflow.db",
			"sqlite",
			"file",
			"/srv/onebase/project",
			8080,
			true,
			cmd,
			true,
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `--sqlite "/var/lib/onebase/docflow.db"`) {
		t.Fatalf("systemd unit must use --sqlite, got:\n%s", out)
	}
	if strings.Contains(out, `--db ""`) {
		t.Fatalf("systemd unit must not include empty --db, got:\n%s", out)
	}
	if !strings.Contains(out, `--project "/srv/onebase/project"`) || !strings.Contains(out, "--watch") {
		t.Fatalf("systemd unit lost project/watch args:\n%s", out)
	}
}
