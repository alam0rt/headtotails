package config

import (
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

// Load reads config from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
