package postgres

import (
	"testing"
	"time"
)

func TestPoolConfig_BoundsConnectionAttempts(t *testing.T) {
	cfg, err := poolConfig("postgres://u:p@localhost:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if cfg.ConnConfig.ConnectTimeout != connectTimeout {
		t.Fatalf("ConnectTimeout = %s, want %s", cfg.ConnConfig.ConnectTimeout, connectTimeout)
	}

	explicit, err := poolConfig("postgres://u:p@localhost:5432/db?sslmode=disable&connect_timeout=2")
	if err != nil {
		t.Fatalf("poolConfig: %v", err)
	}
	if explicit.ConnConfig.ConnectTimeout != 2*time.Second {
		t.Fatalf("an explicit connect_timeout must win, got %s", explicit.ConnConfig.ConnectTimeout)
	}
}
