package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_LLMFromAppYAML проверяет, что секция llm из app.yaml парсится в
// llm.Config и что ${env:VAR} в ключе подставляется из окружения.
func TestLoadConfig_LLMFromAppYAML(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appYAML := `name: Demo
llm:
  enabled: true
  endpoints:
    - name: z_ai
      kind: anthropic
      base_url: https://api.z.ai/api/anthropic
      api_key: "${env:ONEBASE_TEST_LLM_KEY}"
  models:
    - {name: glm-4.6, endpoint: z_ai}
  profiles:
    - {task: чат, models: [glm-4.6]}
  default_profile: чат
`
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte(appYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONEBASE_TEST_LLM_KEY", "secret-123")

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM == nil {
		t.Fatal("ожидался разобранный llm-конфиг, получен nil")
	}
	if !cfg.LLM.Enabled {
		t.Error("llm.enabled должен быть true")
	}
	if len(cfg.LLM.Endpoints) != 1 || cfg.LLM.Endpoints[0].Name != "z_ai" || string(cfg.LLM.Endpoints[0].Kind) != "anthropic" {
		t.Fatalf("endpoints разобраны неверно: %+v", cfg.LLM.Endpoints)
	}
	if got := cfg.LLM.Endpoints[0].APIKey; got != "secret-123" {
		t.Errorf("${env:...} не подставлен: api_key=%q, ожидалось secret-123", got)
	}
	if len(cfg.LLM.Models) != 1 || cfg.LLM.Models[0].Name != "glm-4.6" || cfg.LLM.Models[0].Endpoint != "z_ai" {
		t.Errorf("models разобраны неверно: %+v", cfg.LLM.Models)
	}
	if len(cfg.LLM.Profiles) != 1 || cfg.LLM.Profiles[0].Task != "чат" {
		t.Errorf("profiles разобраны неверно: %+v", cfg.LLM.Profiles)
	}
}

// TestLoadConfig_NoLLMSection — без секции llm поле остаётся nil (поведение баз
// без ИИ-конфига в файле не меняется).
func TestLoadConfig_NoLLMSection(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "app.yaml"), []byte("name: Plain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM != nil {
		t.Errorf("без секции llm ожидался nil, получено %+v", cfg.LLM)
	}
}
