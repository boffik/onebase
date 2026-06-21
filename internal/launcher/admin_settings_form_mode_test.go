package launcher

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/storage"
)

// Проверяем, что настройка глобального режима сохраняется/читается тем же
// механизмом, что использует страница настроек конфигуратора.
func TestConfigurator_FormOpenModeSetting(t *testing.T) {
	db, err := storage.ConnectSQLite(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.SaveFormOpenMode(ctx, storage.FormModeTabs); err != nil {
		t.Fatal(err)
	}
	if got := db.GetFormOpenMode(ctx); got != storage.FormModeTabs {
		t.Errorf("ожидался tabs, получено %q", got)
	}
}
