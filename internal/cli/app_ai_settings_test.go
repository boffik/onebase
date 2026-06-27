package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ivantit66/onebase/internal/project"
	"github.com/ivantit66/onebase/internal/storage"
)

func TestApplyAppAISettings(t *testing.T) {
	ctx := context.Background()
	db, err := storage.ConnectSQLite(ctx, filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	cap := 50000
	errs := applyAppAISettings(ctx, db, &project.AppConfig{
		AI: &project.AIConfig{DataScope: storage.AIDataScopeRBAC, DailyTokenCap: &cap},
	})
	if len(errs) != 0 {
		t.Fatalf("applyAppAISettings returned errors: %v", errs)
	}
	if got := db.GetAIDataScope(ctx); got != storage.AIDataScopeRBAC {
		t.Fatalf("data scope not applied: %q", got)
	}
	if got := db.GetAIDailyTokenCap(ctx); got != cap {
		t.Fatalf("daily token cap not applied: %d", got)
	}
}
