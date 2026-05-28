package metadata

import (
	"path/filepath"
	"testing"
)

func TestLoadHomePage_Rows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "home_page.yaml")
	writeFile(t, path, `title: Главная
layout: rows
rows:
  - widgets: [A, B]
  - widgets: [C]
`)
	hp, err := LoadHomePage(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if hp == nil {
		t.Fatal("hp is nil")
	}
	if hp.Title != "Главная" || hp.Layout != "rows" {
		t.Errorf("unexpected hp: %+v", hp)
	}
	names := hp.WidgetNames()
	if len(names) != 3 || names[0] != "A" || names[1] != "B" || names[2] != "C" {
		t.Errorf("WidgetNames = %v", names)
	}
}

func TestLoadHomePage_Grid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "home_page.yaml")
	writeFile(t, path, `widgets:
  - { name: A, span: 1 }
  - { name: B, span: 3 }
`)
	hp, err := LoadHomePage(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if hp.Layout != "grid" {
		t.Errorf("default layout for flat widgets = %q, want grid", hp.Layout)
	}
	if len(hp.Widgets) != 2 || hp.Widgets[1].Span != 3 {
		t.Errorf("unexpected widgets: %+v", hp.Widgets)
	}
}

func TestLoadHomePage_Missing(t *testing.T) {
	hp, err := LoadHomePage(filepath.Join(t.TempDir(), "no.yaml"))
	if err != nil {
		t.Fatalf("missing should not error: %v", err)
	}
	if hp != nil {
		t.Errorf("missing returned hp = %+v, want nil", hp)
	}
}

func TestApplyDefaults(t *testing.T) {
	// Empty → title "Главная", layout "rows"
	h := &HomePage{}
	h.applyDefaults()
	if h.Title != "Главная" {
		t.Errorf("default title = %q, want %q", h.Title, "Главная")
	}
	if h.Layout != "rows" {
		t.Errorf("default layout (no widgets) = %q, want rows", h.Layout)
	}

	// With flat widgets → layout "grid"
	h2 := &HomePage{Widgets: []HomePageWidget{{Name: "X"}}}
	h2.applyDefaults()
	if h2.Layout != "grid" {
		t.Errorf("default layout (with widgets) = %q, want grid", h2.Layout)
	}

	// Explicit values preserved
	h3 := &HomePage{Title: "Мой стол", Layout: "rows"}
	h3.applyDefaults()
	if h3.Title != "Мой стол" {
		t.Errorf("explicit title overwritten: %q", h3.Title)
	}
	if h3.Layout != "rows" {
		t.Errorf("explicit layout overwritten: %q", h3.Layout)
	}
}
