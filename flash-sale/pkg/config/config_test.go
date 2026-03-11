package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
server:
  port: 9090
database:
  driver: postgres
  host: localhost
  port: 5432
  user: testuser
  password: testpass
  dbname: testdb
  sslmode: disable
  timezone: UTC
redis:
  host: localhost
  port: 6379
  password: ""
  db: 0
jwt:
  secret: test-secret
  expire_hours: 24
`)
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal("LoadConfig() error: %w", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Expected server port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("Expected database driver 'postgres', got '%s'", cfg.Database.Driver)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Expected database host 'localhost', got '%s'", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Expected database port 5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Expected database user 'testuser', got '%s'", cfg.Database.User)
	}
	if cfg.Database.Password != "testpass" {
		t.Errorf("Expected database password 'testpass', got '%s'", cfg.Database.Password)
	}
	if cfg.Database.DBName != "testdb" {
		t.Errorf("Expected database name 'testdb', got '%s'", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("Expected database SSL mode 'disable', got '%s'", cfg.Database.SSLMode)
	}
	if cfg.Database.Timezone != "UTC" {
		t.Errorf("Expected database timezone 'UTC', got '%s'", cfg.Database.Timezone)
	}
	if cfg.Redis.Host != "localhost" {
		t.Errorf("Expected Redis host 'localhost', got '%s'", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6379 {
		t.Errorf("Expected Redis port 6379, got %d", cfg.Redis.Port)
	}
	if cfg.Redis.Password != "" {
		t.Errorf("Expected Redis password '', got '%s'", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("Expected Redis DB 0, got %d", cfg.Redis.DB)
	}
	if cfg.JWT.Secret != "test-secret" {
		t.Errorf("Expected JWT secret 'test-secret', got '%s'", cfg.JWT.Secret)
	}
	if cfg.JWT.ExpireHours != 24 {
		t.Errorf("Expected JWT expire hours 24, got %d", cfg.JWT.ExpireHours)
	}
}
