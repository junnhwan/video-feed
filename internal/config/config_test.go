package config

import "testing"

func TestDefaultConfigUsesLocalServerAndMySQL(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "mysql" {
		t.Fatalf("expected mysql driver, got %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN == "" {
		t.Fatal("expected non-empty database dsn")
	}
	if cfg.Redis.Port != 6379 {
		t.Fatalf("expected redis port 6379, got %d", cfg.Redis.Port)
	}
}
