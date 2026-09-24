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
