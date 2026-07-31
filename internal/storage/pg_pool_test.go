package storage

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func mustParsePoolCfg(t *testing.T, dsn string) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig(%q): %v", dsn, err)
	}
	return cfg
}

// TestApplyPoolDefaults checks the sizing precedence app.yaml > DSN > defaults
// without opening a real connection (ParseConfig does not dial).
func TestApplyPoolDefaults(t *testing.T) {
	const bare = "postgres://u@localhost:5432/db"
	tests := []struct {
		name             string
		dsn              string
		pc               PoolConfig
		wantMax, wantMin int32
	}{
		{"defaults when nothing set", bare, PoolConfig{}, defaultPoolMaxConns, defaultPoolMinConns},
		{"dsn max is honored", bare + "?pool_max_conns=50", PoolConfig{}, 50, defaultPoolMinConns},
		{"dsn min is honored", bare + "?pool_min_conns=5", PoolConfig{}, defaultPoolMaxConns, 5},
		{"app.yaml overrides dsn max", bare + "?pool_max_conns=50", PoolConfig{MaxConns: 30}, 30, defaultPoolMinConns},
		{"app.yaml max and min", bare, PoolConfig{MaxConns: 40, MinConns: 8}, 40, 8},
		{"min clamped to max", bare, PoolConfig{MinConns: 100}, defaultPoolMaxConns, defaultPoolMaxConns},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustParsePoolCfg(t, tt.dsn)
			applyPoolDefaults(cfg, tt.dsn, tt.pc)
			if cfg.MaxConns != tt.wantMax {
				t.Errorf("MaxConns = %d, want %d", cfg.MaxConns, tt.wantMax)
			}
			if cfg.MinConns != tt.wantMin {
				t.Errorf("MinConns = %d, want %d", cfg.MinConns, tt.wantMin)
			}
		})
	}
}
