package storage

// Тест предохранителя сети (план 62): сетевые возможности конфигурации
// (веб-хуки, HTTP-клиент DSL, HTTP-сервисы, email) по умолчанию заблокированы;
// флаг net.enabled в _settings снимает блокировку.

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNetworkEnabled_DefaultLocked(t *testing.T) {
	ctx := context.Background()
	db, err := ConnectSQLite(ctx, filepath.Join(t.TempDir(), "net.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	// Свежая база: сеть заблокирована (предохранитель выключен).
	if db.GetNetworkEnabled(ctx) {
		t.Fatal("по умолчанию сеть должна быть заблокирована")
	}

	if err := db.SaveNetworkEnabled(ctx, true); err != nil {
		t.Fatalf("SaveNetworkEnabled: %v", err)
	}
	if !db.GetNetworkEnabled(ctx) {
		t.Fatal("после включения сеть должна быть разрешена")
	}

	if err := db.SaveNetworkEnabled(ctx, false); err != nil {
		t.Fatalf("SaveNetworkEnabled: %v", err)
	}
	if db.GetNetworkEnabled(ctx) {
		t.Fatal("после выключения сеть снова заблокирована")
	}
}
