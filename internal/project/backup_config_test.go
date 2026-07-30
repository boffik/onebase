package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_BackupSection(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`name: Test
backup:
  enabled: true
  schedule: "0 3 * * *"
  keep_last: 14
  directory: /var/backups/onebase
`)
	if err := os.WriteFile(filepath.Join(dir, "config", "app.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Backup == nil {
		t.Fatal("Backup is nil")
	}
	if !cfg.Backup.Enabled || cfg.Backup.Schedule != "0 3 * * *" || cfg.Backup.KeepLast != 14 {
		t.Fatalf("Backup parsed incorrectly: %+v", cfg.Backup)
	}
	if cfg.Backup.Directory != "/var/backups/onebase" {
		t.Fatalf("directory = %q", cfg.Backup.Directory)
	}
}

func TestLoadConfig_BackupS3EnvExpansion(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`name: Test
backup:
  enabled: true
  s3:
    endpoint: s3.amazonaws.com
    region: eu-central-1
    bucket: my-backups
    prefix: prod/
    access_key: ${env:OB_S3_KEY}
    secret_key: ${env:OB_S3_SECRET}
    keep_last: 30
`)
	if err := os.WriteFile(filepath.Join(dir, "config", "app.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OB_S3_KEY", "AKIAEXAMPLE")
	t.Setenv("OB_S3_SECRET", "topsecret")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Backup == nil || cfg.Backup.S3 == nil {
		t.Fatalf("S3 section missing: %+v", cfg.Backup)
	}
	s3 := cfg.Backup.S3
	if s3.Bucket != "my-backups" || s3.Region != "eu-central-1" || s3.Prefix != "prod/" || s3.KeepLast != 30 {
		t.Fatalf("S3 parsed incorrectly: %+v", s3)
	}
	// Secrets must be resolved from the environment, not left as ${env:...}.
	if s3.AccessKey != "AKIAEXAMPLE" {
		t.Errorf("access_key = %q, want resolved AKIAEXAMPLE", s3.AccessKey)
	}
	if s3.SecretKey != "topsecret" {
		t.Errorf("secret_key = %q, want resolved topsecret", s3.SecretKey)
	}
}
