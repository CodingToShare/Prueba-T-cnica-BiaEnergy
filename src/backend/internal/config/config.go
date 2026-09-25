// Package config loads runtime configuration from environment variables and
// fails fast with actionable messages when it is incomplete or invalid.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"

	"bia-energy.local/backend/internal/auth"
)

// Environment variable names.
const (
	EnvHTTPAddr    = "HTTP_ADDR"
	EnvDatabaseURL = "DATABASE_URL"
	EnvLogLevel    = "LOG_LEVEL"
)

const defaultHTTPAddr = ":8080"

// Config is the runtime configuration shared by the API and the command-line tools.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
}

// Load reads the configuration using getenv (usually os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:    strings.TrimSpace(getenv(EnvHTTPAddr)),
		DatabaseURL: strings.TrimSpace(getenv(EnvDatabaseURL)),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}

	var problems []error
	if err := validateHTTPAddr(cfg.HTTPAddr); err != nil {
		problems = append(problems, err)
	}
	if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		problems = append(problems, err)
	}
	if raw := strings.TrimSpace(getenv(EnvLogLevel)); raw != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(raw)); err != nil {
			problems = append(problems, fmt.Errorf("%s: %q is not a valid level (use debug, info, warn or error)", EnvLogLevel, raw))
		}
	}
	if len(problems) > 0 {
		return Config{}, errors.Join(problems...)
	}
	return cfg, nil
}

// RedactedDatabaseURL returns the database URL with any password masked, for logging.
func (c Config) RedactedDatabaseURL() string {
	u, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "<unparseable>"
	}
	return u.Redacted()
}

// validateHTTPAddr accepts "host:port" or ":port" with a port from 0 to 65535
// (0 lets the OS choose, which tests use).
func validateHTTPAddr(addr string) error {
	_, portText, err := net.SplitHostPort(addr)
	if err == nil {
		if port, perr := strconv.Atoi(portText); perr == nil && port >= 0 && port <= 65535 {
			return nil
		}
	}
	return fmt.Errorf("%s: %q must be host:port or :port with a port between 0 and 65535 (for example :8080)", EnvHTTPAddr, addr)
}

func validateDatabaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%s is required (for example postgres://user:password@localhost:5432/dbname?sslmode=disable)", EnvDatabaseURL)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return fmt.Errorf("%s must be a postgres:// or postgresql:// URL with a host", EnvDatabaseURL)
	}
	return nil
}

// Environment variables read only by the API.
const (
	EnvDemoUsername      = "DEMO_AUTH_USERNAME"
	EnvDemoPassword      = "DEMO_AUTH_PASSWORD"
	EnvSessionSigningKey = "SESSION_SIGNING_KEY"
	EnvSessionSecure     = "SESSION_COOKIE_SECURE"
)

// APIConfig is the API's configuration: the shared settings plus the demo
// authentication (OD-12).
type APIConfig struct {
	Config
	Auth auth.Config
}

// LoadAPI reads the shared configuration and the authentication settings.
// Every problem is reported at once; secret values are never echoed.
func LoadAPI(getenv func(string) string) (APIConfig, error) {
	var problems []error
	base, err := Load(getenv)
	if err != nil {
		problems = append(problems, err)
	}
	cfg := APIConfig{
		Config: base,
		Auth: auth.Config{
			Username:     getenv(EnvDemoUsername),
			Password:     getenv(EnvDemoPassword),
			SigningKey:   []byte(getenv(EnvSessionSigningKey)),
			SecureCookie: true,
		},
	}
	if raw := strings.TrimSpace(getenv(EnvSessionSecure)); raw != "" {
		secure, perr := strconv.ParseBool(raw)
		if perr != nil {
			problems = append(problems, fmt.Errorf("%s: %q must be true or false", EnvSessionSecure, raw))
		}
		cfg.Auth.SecureCookie = secure
	}
	if err := cfg.Auth.Validate(); err != nil {
		problems = append(problems, fmt.Errorf("%s, %s and %s: %w", EnvDemoUsername, EnvDemoPassword, EnvSessionSigningKey, err))
	}
	if len(problems) > 0 {
		return APIConfig{}, errors.Join(problems...)
	}
	return cfg, nil
}
