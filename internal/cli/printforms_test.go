package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLegacyForm = `name: Накладная
document: Реализация
title: "Накладная № {{Номер}}"
header: |
  **Поставщик**: {{Поставщик.Наименование}}
table:
  source: Товары
  columns:
    - field: "@row"
      label: "№"
      width: 40px
    - field: Сумма
      label: Сумма
      align: right
      format: "number:2"
  totals:
    - field: Сумма
      sum: true
      label: Итого
footer: |
  ___

  **Всего**: {{Сумма | money}} руб.
`

func TestMigrateLegacyPrintForms_ConvertsAndDeletes(t *testing.T) {
	dir := t.TempDir()
	pfDir := filepath.Join(dir, "printforms")
	if err := os.MkdirAll(pfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(pfDir, "накладная.yaml")
	if err := os.WriteFile(src, []byte(sampleLegacyForm), 0o644); err != nil {
		t.Fatal(err)
	}

	converted, errs := migrateLegacyPrintForms(dir, false)
	if len(errs) > 0 {
		t.Fatalf("migrate: %v", errs)
	}
	if len(converted) != 1 {
		t.Fatalf("ожидалась 1 конвертированная форма, получили %d", len(converted))
	}

	// Старый .yaml удалён.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("старый .yaml не удалён")
	}
	// Новый .layout.yaml создан.
	out := filepath.Join(pfDir, "накладная.layout.yaml")
	data, ioErr := os.ReadFile(out)
	if ioErr != nil {
		t.Fatalf("новый .layout.yaml не создан: %v", ioErr)
	}
	s := string(data)
	if !strings.Contains(s, "areas:") {
		t.Errorf("в .layout.yaml нет areas:\n%s", s)
	}
	if !strings.Contains(s, "binding:") {
		t.Errorf("в .layout.yaml нет binding:\n%s", s)
	}
	if strings.Contains(s, "| money") {
		t.Errorf("money не сконвертирован в .layout.yaml:\n%s", s)
	}
}

func TestMigrateLegacyPrintForms_KeepFlag(t *testing.T) {
	dir := t.TempDir()
	pfDir := filepath.Join(dir, "printforms")
	_ = os.MkdirAll(pfDir, 0o755)
	src := filepath.Join(pfDir, "накладная.yaml")
	_ = os.WriteFile(src, []byte(sampleLegacyForm), 0o644)

	if _, errs := migrateLegacyPrintForms(dir, true); len(errs) > 0 {
		t.Fatalf("migrate --keep: %v", errs)
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("с --keep старый .yaml должен сохраниться")
	}
	if _, err := os.Stat(filepath.Join(pfDir, "накладная.layout.yaml")); err != nil {
		t.Error("новый .layout.yaml не создан")
	}
}

// .os-формы и уже существующие .layout.yaml не трогаются.
func TestMigrateLegacyPrintForms_SkipsNonLegacy(t *testing.T) {
	dir := t.TempDir()
	pfDir := filepath.Join(dir, "printforms")
	_ = os.MkdirAll(pfDir, 0o755)
	osForm := filepath.Join(pfDir, "печать.os")
	_ = os.WriteFile(osForm, []byte("// Документ: Док\nПроцедура Сформировать()\nКонецПроцедуры"), 0o644)
	layoutForm := filepath.Join(pfDir, "готовая.layout.yaml")
	_ = os.WriteFile(layoutForm, []byte("name: Готовая\nareas: []\n"), 0o644)

	converted, errs := migrateLegacyPrintForms(dir, false)
	if len(errs) > 0 {
		t.Fatalf("migrate: %v", errs)
	}
	if len(converted) != 0 {
		t.Errorf("не legacy-формы не должны конвертироваться, got %v", converted)
	}
	if _, err := os.Stat(osForm); err != nil {
		t.Error(".os форма удалена/тронута")
	}
	if _, err := os.Stat(layoutForm); err != nil {
		t.Error(".layout.yaml удалён/тронут")
	}
}

// Битый файл не ломает миграцию остальных: валидная форма конвертируется,
// ошибка по битой попадает в срез errs, повторный запуск доделает остальное.
func TestMigrateLegacyPrintForms_ContinuesOnError(t *testing.T) {
	dir := t.TempDir()
	pfDir := filepath.Join(dir, "printforms")
	_ = os.MkdirAll(pfDir, 0o755)

	// Валидная legacy-форма.
	_ = os.WriteFile(filepath.Join(pfDir, "valid.yaml"), []byte(sampleLegacyForm), 0o644)
	// Битая форма: не YAML.
	_ = os.WriteFile(filepath.Join(pfDir, "broken.yaml"), []byte("::not yaml::\n\tbroken:\t"), 0o644)

	converted, errs := migrateLegacyPrintForms(dir, false)

	// Валидная форма должна быть сконвертирована.
	if len(converted) != 1 || converted[0].From != "valid.yaml" {
		t.Errorf("ожидалась 1 конвертированная форма (valid.yaml), получили %v", converted)
	}
	// Ошибка по битой должна быть в errs.
	if len(errs) == 0 {
		t.Error("ожидалась ошибка для broken.yaml, errs пуст")
	}
	hasBrokenErr := false
	for _, e := range errs {
		if e.File == "broken.yaml" {
			hasBrokenErr = true
		}
	}
	if !hasBrokenErr {
		t.Errorf("ожидалась ошибка для broken.yaml, получили: %v", errs)
	}
	// Валидный .layout.yaml создан.
	if _, err := os.Stat(filepath.Join(pfDir, "valid.layout.yaml")); err != nil {
		t.Error("valid.layout.yaml не создан")
	}
}

// Нет каталога printforms — не ошибка, пустой результат.
func TestMigrateLegacyPrintForms_NoDir(t *testing.T) {
	dir := t.TempDir()
	converted, errs := migrateLegacyPrintForms(dir, false)
	if len(errs) > 0 {
		t.Fatalf("migrate без printforms: %v", errs)
	}
	if len(converted) != 0 {
		t.Errorf("ожидался пустой результат, got %v", converted)
	}
}
