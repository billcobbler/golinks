package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the golinks server.
// Values are loaded from environment variables with sensible defaults.
type Config struct {
	Port               string
	DBPath             string
	AuthMode           string // none | local | oauth | local+oauth
	OAuthProvider      string // google | github
	OAuthClientID      string
	OAuthClientSecret  string
	BaseURL            string
	LogLevel           string // debug | info | warn | error
	AnalyticsRetention int    // days to keep click history; 0 = indefinite
	// InsecureCookies disables the Secure flag on session cookies.
	// Set GOLINKS_INSECURE_COOKIES=true when running without TLS.
	InsecureCookies bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("GOLINKS_PORT", "8080"),
		DBPath:            getEnv("GOLINKS_DB", "./golinks.db"),
		AuthMode:          getEnv("GOLINKS_AUTH", "none"),
		OAuthProvider:     getEnv("GOLINKS_OAUTH_PROVIDER", ""),
		OAuthClientID:     getEnv("GOLINKS_OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: getEnv("GOLINKS_OAUTH_CLIENT_SECRET", ""),
		BaseURL:           getEnv("GOLINKS_BASE_URL", ""),
		LogLevel:          getEnv("GOLINKS_LOG_LEVEL", "info"),
		InsecureCookies:   getEnv("GOLINKS_INSECURE_COOKIES", "") == "true",
	}

	retentionStr := getEnv("GOLINKS_ANALYTICS_RETENTION", "0")
	retention, err := strconv.Atoi(retentionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid GOLINKS_ANALYTICS_RETENTION %q: must be an integer (days)", retentionStr)
	}
	cfg.AnalyticsRetention = retention

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	validAuthModes := map[string]bool{"none": true, "local": true, "oauth": true, "local+oauth": true}
	if !validAuthModes[c.AuthMode] {
		return fmt.Errorf("invalid GOLINKS_AUTH %q: must be one of: none, local, oauth, local+oauth", c.AuthMode)
	}

	if c.AuthMode == "oauth" || c.AuthMode == "local+oauth" {
		if c.OAuthProvider == "" {
			return fmt.Errorf("GOLINKS_OAUTH_PROVIDER is required when auth mode includes oauth")
		}
		if c.OAuthClientID == "" || c.OAuthClientSecret == "" {
			return fmt.Errorf("GOLINKS_OAUTH_CLIENT_ID and GOLINKS_OAUTH_CLIENT_SECRET are required when auth mode includes oauth")
		}
		validProviders := map[string]bool{"google": true, "github": true}
		if !validProviders[c.OAuthProvider] {
			return fmt.Errorf("invalid GOLINKS_OAUTH_PROVIDER %q: must be one of: google, github", c.OAuthProvider)
		}
		if c.BaseURL == "" {
			return fmt.Errorf("GOLINKS_BASE_URL is required when auth mode includes oauth (used for the OAuth callback URL)")
		}
	}

	if c.AnalyticsRetention < 0 {
		return fmt.Errorf("GOLINKS_ANALYTICS_RETENTION must be >= 0 (0 = keep forever)")
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
