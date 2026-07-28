package launcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestCfgAdminUsers_FirstUserWarnsAndChecksAdmin(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "first-user.db")
	db, err := storage.ConnectSQLite(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	repo := auth.NewRepo(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	t.Cleanup(CloseAuthPools)

	store := &Store{path: filepath.Join(t.TempDir(), "ibases.yaml")}
	base := &Base{ID: "first-user-form", ConfigSource: "database", DBType: "sqlite", DBPath: dbPath}
	if err := store.save([]*Base{base}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bases/first-user-form/configurator/admin/users", nil)
	req = requestWithBaseID(req, base.ID)
	rec := httptest.NewRecorder()
	(&handler{store: store, runner: NewRunner()}).cfgAdminUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Первый пользователь должен быть администратором") {
		t.Error("панель первого пользователя должна содержать предупреждение")
	}
	if !strings.Contains(body, `id="cfg-ua" checked`) {
		t.Error("чекбокс Админ должен быть отмечен по умолчанию")
	}
}
