package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoadReadsYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, `
server:
  port: 9090
database:
  driver: mysql
  dsn: user:pass@tcp(db:3306)/app?parseTime=true
redis:
  host: redis
  port: 6380
  password: secret
  db: 2
rabbitmq:
  host: rabbitmq
  port: 5673
  username: guest
  password: guest
observability:
  pprof:
    enabled: true
    api_addr: 127.0.0.1:6060
    worker_addr: 127.0.0.1:6061
`)

	cfg, err := Load(path)

	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Database.DSN == "" || cfg.Redis.DB != 2 {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	if !cfg.Observability.Pprof.Enabled || cfg.Observability.Pprof.WorkerAddr != "127.0.0.1:6061" {
		t.Fatalf("unexpected observability config: %+v", cfg.Observability)
	}
}

func writeConfigFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
