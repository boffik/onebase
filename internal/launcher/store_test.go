package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{path: filepath.Join(t.TempDir(), "ibases.yaml")}
}

func TestStore_EmptyList(t *testing.T) {
	s := newTestStore(t)
	bases, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bases) != 0 {
		t.Fatalf("want 0 bases, got %d", len(bases))
	}
}

func TestStore_AddAndList(t *testing.T) {
	s := newTestStore(t)

	b := &Base{Name: "Склад", DB: "postgres://localhost/sklad", Port: 8080}
	if err := s.Add(b); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if b.ID == "" {
		t.Fatal("ID should be auto-set by Add")
	}
	if b.Created.IsZero() {
		t.Fatal("Created should be auto-set by Add")
	}
	if b.ConfigSource == "" {
		t.Fatal("ConfigSource should be defaulted by Add")
	}

	bases, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bases) != 1 {
		t.Fatalf("want 1 base, got %d", len(bases))
	}
	if bases[0].Name != "Склад" {
		t.Fatalf("want Name=Склад, got %q", bases[0].Name)
	}
}

func TestStore_Get(t *testing.T) {
	s := newTestStore(t)
	b := &Base{Name: "ERP", DB: "postgres://localhost/erp"}
	if err := s.Add(b); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get(b.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "ERP" {
		t.Fatalf("want ERP, got %q", got.Name)
	}

	_, err = s.Get("nonexistent-id")
	if err == nil {
		t.Fatal("Get with unknown ID should return error")
	}
}

func TestStore_Update(t *testing.T) {
	s := newTestStore(t)
	b := &Base{Name: "Old", DB: "postgres://localhost/old"}
	if err := s.Add(b); err != nil {
		t.Fatal(err)
	}

	b.Name = "New"
	b.Port = 9090
	if err := s.Update(b); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(b.ID)
	if got.Name != "New" {
		t.Fatalf("want New, got %q", got.Name)
	}
	if got.Port != 9090 {
		t.Fatalf("want 9090, got %d", got.Port)
	}
}

func TestStore_Remove(t *testing.T) {
	s := newTestStore(t)
	b1 := &Base{Name: "A", DB: "postgres://localhost/a"}
	b2 := &Base{Name: "B", DB: "postgres://localhost/b"}
	if err := s.Add(b1); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(b2); err != nil {
		t.Fatal(err)
	}

	if err := s.Remove(b1.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	bases, _ := s.List()
	if len(bases) != 1 {
		t.Fatalf("want 1 after remove, got %d", len(bases))
	}
	if bases[0].Name != "B" {
		t.Fatalf("wrong base left after remove: %q", bases[0].Name)
	}
}

func TestStore_Move(t *testing.T) {
	s := newTestStore(t)
	a := &Base{Name: "A", DB: "postgres://localhost/a"}
	b := &Base{Name: "B", DB: "postgres://localhost/b"}
	c := &Base{Name: "C", DB: "postgres://localhost/c"}
	if err := s.Add(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(b); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(c); err != nil {
		t.Fatal(err)
	}

	order := func() []string {
		bases, _ := s.List()
		names := make([]string, len(bases))
		for i, x := range bases {
			names[i] = x.Name
		}
		return names
	}

	// Move B up: B,A,C
	if err := s.Move(b.ID, -1); err != nil {
		t.Fatalf("Move up: %v", err)
	}
	if got := order(); got[0] != "B" || got[1] != "A" || got[2] != "C" {
		t.Fatalf("after move up want B,A,C got %v", got)
	}

	// Move B down twice: A,C,B
	if err := s.Move(b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Move(b.ID, 1); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != "A" || got[1] != "C" || got[2] != "B" {
		t.Fatalf("after move down want A,C,B got %v", got)
	}

	// Move past the bottom edge — no-op
	if err := s.Move(b.ID, 1); err != nil {
		t.Fatalf("Move at edge: %v", err)
	}
	if got := order(); got[2] != "B" {
		t.Fatalf("edge move should be no-op, got %v", got)
	}

	// Unknown id — error
	if err := s.Move("nope", -1); err == nil {
		t.Fatal("Move with unknown id should error")
	}
}

func TestStore_AtomicWrite(t *testing.T) {
	s := newTestStore(t)
	b := &Base{Name: "Atomic", DB: "postgres://localhost/atomic"}
	if err := s.Add(b); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// The .tmp file should not exist — it was renamed to the final path
	tmpPath := s.path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Fatal(".tmp file should be cleaned up after atomic write")
	}

	// The actual file should exist
	if _, err := os.Stat(s.path); err != nil {
		t.Fatalf("ibases.yaml not found: %v", err)
	}
}

func TestStore_MultipleOps_Persistence(t *testing.T) {
	s := newTestStore(t)

	for i := range 3 {
		if err := s.Add(&Base{
			Name: []string{"Alpha", "Beta", "Gamma"}[i],
			DB:   "postgres://localhost/db",
		}); err != nil {
			t.Fatal(err)
		}
	}

	bases, _ := s.List()
	if len(bases) != 3 {
		t.Fatalf("want 3 bases, got %d", len(bases))
	}

	// Reload from same file to verify persistence
	s2 := &Store{path: s.path}
	bases2, _ := s2.List()
	if len(bases2) != 3 {
		t.Fatalf("persisted store should have 3 bases, got %d", len(bases2))
	}
}
