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
	"time"

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

	EnvExplanationProvider = "EXPLANATION_PROVIDER"
	EnvOllamaBaseURL       = "OLLAMA_BASE_URL"
	EnvOllamaModel         = "OLLAMA_MODEL"
	EnvOllamaTimeout       = "OLLAMA_TIMEOUT"
)

// Explanation providers (ADR-006).
const (
	ProviderDeterministic = "deterministic"
	ProviderOllama        = "ollama"
)

// Explanation defaults. The Ollama URL is local; a remote service is only
// used when configured explicitly.
const (
	defaultOllamaBaseURL = "http://127.0.0.1:11434"
	defaultOllamaTimeout = 60 * time.Second
	maxOllamaTimeout     = 5 * time.Minute
)

// ExplanationConfig selects how findings are explained. The deterministic
// provider is the default and needs nothing else.
type ExplanationConfig struct {
	Provider      string
	OllamaBaseURL string
	OllamaModel   string
	OllamaTimeout time.Duration
}

// APIConfig is the API's configuration: the shared settings plus the demo
// authentication (OD-12) and the explanation provider.
type APIConfig struct {
	Config
	Auth        auth.Config
	Explanation ExplanationConfig
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
	var explanationProblems []error
	cfg.Explanation, explanationProblems = loadExplanation(getenv)
	problems = append(problems, explanationProblems...)
	if len(problems) > 0 {
		return APIConfig{}, errors.Join(problems...)
	}
	return cfg, nil
}

// loadExplanation reads the explanation settings. The Ollama settings are
// validated only when that provider is selected.
func loadExplanation(getenv func(string) string) (ExplanationConfig, []error) {
	cfg := ExplanationConfig{
		Provider:      strings.ToLower(strings.TrimSpace(getenv(EnvExplanationProvider))),
		OllamaBaseURL: strings.TrimSpace(getenv(EnvOllamaBaseURL)),
		OllamaModel:   strings.TrimSpace(getenv(EnvOllamaModel)),
		OllamaTimeout: defaultOllamaTimeout,
	}
	if cfg.Provider == "" {
		cfg.Provider = ProviderDeterministic
	}
	if cfg.OllamaBaseURL == "" {
		cfg.OllamaBaseURL = defaultOllamaBaseURL
	}
	var problems []error
	switch cfg.Provider {
	case ProviderDeterministic:
		return cfg, nil
	case ProviderOllama:
	default:
		return cfg, []error{fmt.Errorf("%s: %q must be %s or %s", EnvExplanationProvider, cfg.Provider, ProviderDeterministic, ProviderOllama)}
	}
	if u, err := url.Parse(cfg.OllamaBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		problems = append(problems, fmt.Errorf("%s must be an http(s) URL with a host and no credentials (for example %s)", EnvOllamaBaseURL, defaultOllamaBaseURL))
	}
	if cfg.OllamaModel == "" {
		problems = append(problems, fmt.Errorf("%s is required when %s=%s (for example llama3.2:3b)", EnvOllamaModel, EnvExplanationProvider, ProviderOllama))
	}
	if raw := strings.TrimSpace(getenv(EnvOllamaTimeout)); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 || d > maxOllamaTimeout {
			problems = append(problems, fmt.Errorf("%s: %q must be a positive duration of at most %s (for example 60s)", EnvOllamaTimeout, raw, maxOllamaTimeout))
		} else {
			cfg.OllamaTimeout = d
		}
	}
	return cfg, problems
}
