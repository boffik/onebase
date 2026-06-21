package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := ConnectSQLite(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestFormOpenMode_GlobalDefaultAndSave(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	// Без ключа — дефолт pages.
	if got := db.GetFormOpenMode(ctx); got != FormModePages {
		t.Errorf("дефолт: ожидался %q, получено %q", FormModePages, got)
	}
	// Сохранили tabs — читается tabs.
	if err := db.SaveFormOpenMode(ctx, FormModeTabs); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := db.GetFormOpenMode(ctx); got != FormModeTabs {
		t.Errorf("после save tabs: получено %q", got)
	}
	// Битое значение → pages.
	if err := db.SaveFormOpenMode(ctx, "мусор"); err != nil {
		t.Fatalf("save мусор: %v", err)
	}
	if got := db.GetFormOpenMode(ctx); got != FormModePages {
		t.Errorf("битое значение должно дать %q, получено %q", FormModePages, got)
	}
}
