package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // Windows

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Server != "http://localhost:8080" {
		t.Errorf("Server = %q, want %q", cfg.Server, "http://localhost:8080")
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
}

func TestSetAndGetConfigValue(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if err := SetConfigValue("server", "http://myserver:9090"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Server != "http://myserver:9090" {
		t.Errorf("Server = %q, want %q", cfg.Server, "http://myserver:9090")
	}

	val, err := GetConfigValue("server")
	if err != nil {
		t.Fatalf("GetConfigValue: %v", err)
	}
	if val != "http://myserver:9090" {
		t.Errorf("GetConfigValue = %q, want %q", val, "http://myserver:9090")
	}
}

func TestSetConfigValue_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	if err := SetConfigValue("token", "mytoken123"); err != nil {
		t.Fatalf("SetConfigValue: %v", err)
	}

	cfgFile := filepath.Join(tmp, ".config", "golinks", "config.yaml")
	if _, err := os.Stat(cfgFile); err != nil {
		t.Errorf("config file not created: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Token != "mytoken123" {
		t.Errorf("Token = %q, want %q", cfg.Token, "mytoken123")
	}
}

func TestGetConfigValue_UnknownKey(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	_, err := GetConfigValue("nonexistent")
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}
