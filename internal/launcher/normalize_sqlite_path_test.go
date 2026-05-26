package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeSQLitePath(t *testing.T) {
	tmp := t.TempDir()
	existingDir := filepath.Join(tmp, "exists")
	if err := os.MkdirAll(existingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		path     string
		baseName string
		want     string
	}{
		{"empty path stays empty", "", "MyBase", ""},
		{"already .db preserved", `C:\bases\foo.db`, "MyBase", `C:\bases\foo.db`},
		{"already .DB preserved", `C:\bases\foo.DB`, "MyBase", `C:\bases\foo.DB`},
		{
			"folder path without extension gets <name>.db",
			`C:\OneBases\test`, "MyBase",
			filepath.Join(`C:\OneBases\test`, "MyBase.db"),
		},
		{
			"trailing backslash treated as folder",
			`C:\OneBases\test\`, "MyBase",
			filepath.Join(`C:\OneBases\test`, "MyBase.db"),
		},
		{
			"trailing slash treated as folder",
			"/var/data/", "MyBase",
			filepath.Join("/var/data", "MyBase.db"),
		},
		{
			"existing dir on disk treated as folder",
			existingDir, "MyBase",
			filepath.Join(existingDir, "MyBase.db"),
		},
		{
			"unsafe chars in name sanitized",
			`C:\OneBases\test`, `bad/name:?`,
			filepath.Join(`C:\OneBases\test`, "bad_name__.db"),
		},
		{
			"empty name falls back to database",
			`C:\OneBases\test`, "",
			filepath.Join(`C:\OneBases\test`, "database.db"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeSQLitePath(tc.path, tc.baseName)
			if got != tc.want {
				t.Fatalf("normalizeSQLitePath(%q, %q) = %q, want %q",
					tc.path, tc.baseName, got, tc.want)
			}
		})
	}
}
