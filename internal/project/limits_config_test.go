package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Limits(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`
name: Test
limits:
  request_timeout_sec: 10
  report_timeout_sec: 20
  report_max_rows: 1000
  export_timeout_sec: 30
  export_max_rows: 2000
  processor_timeout_sec: 40
  processor_concurrency: 2
  http_service_timeout_sec: 50
  http_service_concurrency: 3
  slow_operation_ms: 1500
`)
	if err := os.WriteFile(filepath.Join(dir, "config", "app.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limits == nil {
		t.Fatal("Limits nil")
	}
	if cfg.Limits.ReportMaxRows != 1000 || cfg.Limits.HTTPServiceConcurrency != 3 || cfg.Limits.SlowOperationMS != 1500 {
		t.Fatalf("limits parsed incorrectly: %+v", cfg.Limits)
	}
}
