package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEntity_DisplayName(t *testing.T) {
	tests := []struct {
		name   string
		entity Entity
		lang   string
		want   string
	}{
		{
			name:   "empty entity falls back to Name",
			entity: Entity{Name: "Контрагенты"},
			lang:   "en",
			want:   "Контрагенты",
		},
		{
			name:   "Title used when Titles is empty",
			entity: Entity{Name: "Контрагенты", Title: "Контрагенты компании"},
			lang:   "en",
			want:   "Контрагенты компании",
		},
		{
			name: "Titles[lang] wins over Title",
			entity: Entity{
				Name:   "Контрагенты",
				Title:  "Контрагенты компании",
				Titles: map[string]string{"en": "Counterparties", "sr": "Партнери"},
			},
			lang: "en",
			want: "Counterparties",
		},
		{
			name: "missing lang in Titles falls back to Title",
			entity: Entity{
				Name:   "Контрагенты",
				Title:  "Контрагенты компании",
				Titles: map[string]string{"en": "Counterparties"},
			},
			lang: "de",
			want: "Контрагенты компании",
		},
		{
			name: "empty translation in Titles is ignored (falls back)",
			entity: Entity{
				Name:   "Контрагенты",
				Title:  "Контрагенты компании",
				Titles: map[string]string{"en": ""},
			},
			lang: "en",
			want: "Контрагенты компании",
		},
		{
			name: "empty lang skips Titles lookup",
			entity: Entity{
				Name:   "Контрагенты",
				Title:  "Контрагенты компании",
				Titles: map[string]string{"en": "Counterparties"},
			},
			lang: "",
			want: "Контрагенты компании",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entity.DisplayName(tt.lang)
			if got != tt.want {
				t.Errorf("DisplayName(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestLoadFile_Titles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cat.yaml")
	yaml := `name: Контрагенты
title: Контрагенты компании
titles:
  en: Counterparties
  sr: Партнери
fields:
  - name: ИНН
    type: string
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := LoadFile(path, KindCatalog)
	if err != nil {
		t.Fatal(err)
	}
	if e.Title != "Контрагенты компании" {
		t.Errorf("Title = %q, want %q", e.Title, "Контрагенты компании")
	}
	if got := e.Titles["en"]; got != "Counterparties" {
		t.Errorf("Titles[en] = %q, want %q", got, "Counterparties")
	}
	if got := e.Titles["sr"]; got != "Партнери" {
		t.Errorf("Titles[sr] = %q, want %q", got, "Партнери")
	}
	if got := e.DisplayName("en"); got != "Counterparties" {
		t.Errorf("DisplayName(en) = %q, want %q", got, "Counterparties")
	}
}
