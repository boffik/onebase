package configdb_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/ivantit66/onebase/internal/configdb"
)

func TestRepoVersions_SaveDiffRollback(t *testing.T) {
	repo, _, ctx := newSQLiteRepo(t)

	if err := repo.SaveFile(ctx, "config/app.yaml", []byte("name: v1\n")); err != nil {
		t.Fatalf("SaveFile v1: %v", err)
	}
	versions, err := repo.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("versions len = %d, want 1", len(versions))
	}
	baseline := versions[0]
	if baseline.Message != "save config/app.yaml" {
		t.Fatalf("baseline message = %q", baseline.Message)
	}

	if err := repo.SaveFile(ctx, "config/app.yaml", []byte("name: v2\n")); err != nil {
		t.Fatalf("SaveFile v2: %v", err)
	}
	if err := repo.SaveFile(ctx, "reports/sales.yaml", []byte("name: sales\n")); err != nil {
		t.Fatalf("SaveFile report: %v", err)
	}
	versions, err = repo.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions after changes: %v", err)
	}
	latest := versions[0]

	diff, err := repo.DiffVersions(ctx, baseline.ID, latest.ID)
	if err != nil {
		t.Fatalf("DiffVersions: %v", err)
	}
	want := map[string]configdb.DiffKind{
		"config/app.yaml":    configdb.DiffModified,
		"reports/sales.yaml": configdb.DiffAdded,
	}
	if len(diff) != len(want) {
		t.Fatalf("diff = %+v", diff)
	}
	for _, d := range diff {
		if want[d.Path] != d.Kind {
			t.Fatalf("diff entry = %+v", d)
		}
	}

	rolled, err := repo.RollbackToVersion(ctx, baseline.ID, configdb.VersionOptions{Message: "rollback test"})
	if err != nil {
		t.Fatalf("RollbackToVersion: %v", err)
	}
	if rolled.ID == "" || rolled.ID == baseline.ID || rolled.Message != "rollback test" {
		t.Fatalf("rollback version = %+v", rolled)
	}
	content, ok, err := repo.ReadFile(ctx, "config/app.yaml")
	if err != nil || !ok {
		t.Fatalf("ReadFile app: ok=%v err=%v", ok, err)
	}
	if !bytes.Equal(content, []byte("name: v1\n")) {
		t.Fatalf("rolled back content = %q", content)
	}
	if _, ok, err := repo.ReadFile(ctx, "reports/sales.yaml"); err != nil || ok {
		t.Fatalf("report should be absent after rollback: ok=%v err=%v", ok, err)
	}
}

func TestRepoVersions_DeleteCreatesVersion(t *testing.T) {
	repo, _, ctx := newSQLiteRepo(t)
	if err := repo.SaveFile(ctx, "src/a.os", []byte("Процедура X()\nКонецПроцедуры\n")); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	versions, _ := repo.ListVersions(ctx, 10)
	beforeDelete := versions[0]

	if err := repo.DeleteFile(ctx, "src/a.os"); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	versions, err := repo.ListVersions(ctx, 10)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("versions len = %d, want 2", len(versions))
	}
	if versions[0].Message != "delete src/a.os" {
		t.Fatalf("delete message = %q", versions[0].Message)
	}
	diff, err := repo.DiffVersions(ctx, beforeDelete.ID, versions[0].ID)
	if err != nil {
		t.Fatalf("DiffVersions delete: %v", err)
	}
	if len(diff) != 1 || diff[0].Path != "src/a.os" || diff[0].Kind != configdb.DiffDeleted {
		t.Fatalf("delete diff = %+v", diff)
	}
}

func TestRepoVersions_DeleteMissingDoesNotCreateVersion(t *testing.T) {
	repo, _, ctx := newSQLiteRepo(t)
	if err := repo.DeleteFile(ctx, "missing.yaml"); err != nil {
		t.Fatalf("DeleteFile missing: %v", err)
	}
	versions, err := repo.ListVersions(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 0 {
		t.Fatalf("versions after no-op delete = %+v", versions)
	}
}
