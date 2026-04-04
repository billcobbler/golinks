package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the CLI configuration.
type Config struct {
	Server string
	Token  string
}

// ConfigDir returns the directory where the config file is stored.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".config", "golinks"), nil
}

// LoadConfig reads the config file, returning defaults if it doesn't exist.
func LoadConfig() (*Config, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)
	v.SetDefault("server", "http://localhost:8080")
	v.SetDefault("token", "")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	return &Config{
		Server: v.GetString("server"),
		Token:  v.GetString("token"),
	}, nil
}

// SetConfigValue writes a single key=value pair to the config file.
func SetConfigValue(key, value string) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(dir)
	v.SetDefault("server", "http://localhost:8080")
	v.SetDefault("token", "")
	_ = v.ReadInConfig()

	v.Set(key, value)
	return v.WriteConfigAs(filepath.Join(dir, "config.yaml"))
}

// GetConfigValue returns a single config value by key.
func GetConfigValue(key string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	switch key {
	case "server":
		return cfg.Server, nil
	case "token":
		return cfg.Token, nil
	default:
		return "", fmt.Errorf("unknown config key %q (valid keys: server, token)", key)
	}
}
