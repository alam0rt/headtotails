package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all runtime configuration for headtotails.
type Config struct {
	ListenAddr        string `envconfig:"LISTEN_ADDR" default:":8080"`
	HeadscaleAddr     string `envconfig:"HEADSCALE_ADDR" required:"true"`
	HeadscaleAPIKey   string `envconfig:"HEADSCALE_API_KEY" required:"true"`
	TailnetName       string `envconfig:"TAILNET_NAME" default:"-"`
	OAuthHMACSecret   string `envconfig:"OAUTH_HMAC_SECRET" required:"true"`
	OAuthClientID     string `envconfig:"OAUTH_CLIENT_ID" required:"true"`
	OAuthClientSecret string `envconfig:"OAUTH_CLIENT_SECRET" required:"true"`
	TLSCert           string `envconfig:"TLS_CERT"`
	TLSKey            string `envconfig:"TLS_KEY"`
	LogLevel          string `envconfig:"LOG_LEVEL" default:"info"`
	LogAddSource      bool   `envconfig:"LOG_ADD_SOURCE" default:"false"`
	Environment       string `envconfig:"ENVIRONMENT" default:"production"`
}

// Load reads config from environment variables and validates that
// security-critical fields are non-empty.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate checks that security-critical fields contain non-empty values.
// The envconfig "required" tag only ensures the env var is set — it does
// not reject zero-length strings. An empty credential silently disables
// authentication, so we catch it here at startup.
func (c *Config) validate() error {
	var missing []string
	if strings.TrimSpace(c.HeadscaleAPIKey) == "" {
		missing = append(missing, "HEADSCALE_API_KEY")
	}
	if strings.TrimSpace(c.OAuthClientID) == "" {
		missing = append(missing, "OAUTH_CLIENT_ID")
	}
	if strings.TrimSpace(c.OAuthClientSecret) == "" {
		missing = append(missing, "OAUTH_CLIENT_SECRET")
	}
	if strings.TrimSpace(c.OAuthHMACSecret) == "" {
		missing = append(missing, "OAUTH_HMAC_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("required config values must be non-empty: %s", strings.Join(missing, ", "))
	}
	return nil
}
