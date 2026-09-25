package config

import (
	"log/slog"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad_OnlyDatabaseURL_AppliesDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{EnvDatabaseURL: "postgres://u:p@localhost:5432/db"}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoad_InvalidValues_ReportsEveryProblem(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		want []string
	}{
		"missing database url": {
			env:  map[string]string{},
			want: []string{"DATABASE_URL is required"},
		},
		"wrong scheme": {
			env:  map[string]string{EnvDatabaseURL: "mysql://localhost/db"},
			want: []string{"DATABASE_URL must be a postgres://"},
		},
		"invalid log level and missing url": {
			env:  map[string]string{EnvLogLevel: "verbose"},
			want: []string{"DATABASE_URL is required", `LOG_LEVEL: "verbose" is not a valid level`},
		},
		"http address without port": {
			env:  map[string]string{EnvDatabaseURL: "postgres://u:p@localhost/db", EnvHTTPAddr: "localhost"},
			want: []string{`HTTP_ADDR: "localhost" must be host:port or :port`},
		},
		"http port out of range": {
			env:  map[string]string{EnvDatabaseURL: "postgres://u:p@localhost/db", EnvHTTPAddr: ":70000"},
			want: []string{`HTTP_ADDR: ":70000" must be host:port or :port`},
		},
		"http port not numeric": {
			env:  map[string]string{EnvDatabaseURL: "postgres://u:p@localhost/db", EnvHTTPAddr: ":http-ish"},
			want: []string{`HTTP_ADDR: ":http-ish"`},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(tc.env))
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, fragment := range tc.want {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("error %q does not mention %q", err, fragment)
				}
			}
		})
	}
}

func TestLoad_ValidHTTPAddrForms_AreAccepted(t *testing.T) {
	for _, addr := range []string{":8080", "127.0.0.1:9090", "[::1]:8080", "localhost:0"} {
		cfg, err := Load(env(map[string]string{EnvDatabaseURL: "postgres://u:p@localhost/db", EnvHTTPAddr: addr}))
		if err != nil {
			t.Errorf("HTTP_ADDR %q rejected: %v", addr, err)
			continue
		}
		if cfg.HTTPAddr != addr {
			t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, addr)
		}
	}
}

func TestRedactedDatabaseURL_HidesPassword(t *testing.T) {
	cfg := Config{DatabaseURL: "postgres://bia:s3cret-value@localhost:5432/bia_energy"}

	got := cfg.RedactedDatabaseURL()

	if strings.Contains(got, "s3cret-value") {
		t.Fatalf("redacted URL still contains the password: %q", got)
	}
	if !strings.Contains(got, "localhost:5432/bia_energy") {
		t.Fatalf("redacted URL lost host or database: %q", got)
	}
}

var validAPIEnv = map[string]string{
	EnvDatabaseURL:       "postgres://u:p@localhost/db",
	EnvDemoUsername:      "demo",
	EnvDemoPassword:      "test-only-password",
	EnvSessionSigningKey: "test-only-signing-key-0123456789abcdef",
}

func withEnv(overrides map[string]string) func(string) string {
	m := map[string]string{}
	for k, v := range validAPIEnv {
		m[k] = v
	}
	for k, v := range overrides {
		m[k] = v
	}
	return env(m)
}

func TestLoadAPI_ValidAuthSettings_SecureCookieByDefault(t *testing.T) {
	cfg, err := LoadAPI(withEnv(nil))
	if err != nil {
		t.Fatalf("LoadAPI: %v", err)
	}
	if cfg.Auth.Username != "demo" || !cfg.Auth.SecureCookie {
		t.Fatalf("unexpected auth config: user %q secure %v", cfg.Auth.Username, cfg.Auth.SecureCookie)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("shared configuration not loaded")
	}

	local, err := LoadAPI(withEnv(map[string]string{EnvSessionSecure: "false"}))
	if err != nil || local.Auth.SecureCookie {
		t.Fatalf("SESSION_COOKIE_SECURE=false must disable Secure explicitly (err %v)", err)
	}
}

func TestLoadAPI_MissingOrInvalidAuth_FailsWithoutEchoingSecrets(t *testing.T) {
	_, err := LoadAPI(withEnv(map[string]string{
		EnvDemoUsername:      "",
		EnvDemoPassword:      "short1",
		EnvSessionSigningKey: "leaky-short-key",
		EnvSessionSecure:     "maybe",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{EnvDemoUsername, "SESSION_COOKIE_SECURE", "at least 8 characters", "at least 32 bytes"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not mention %q", msg, want)
		}
	}
	for _, secret := range []string{"short1", "leaky-short-key"} {
		if strings.Contains(msg, secret) {
			t.Errorf("error leaks a secret value: %q", msg)
		}
	}
}

func TestLoad_SharedConfigurationDoesNotRequireAuth(t *testing.T) {
	if _, err := Load(env(map[string]string{EnvDatabaseURL: "postgres://u:p@localhost/db"})); err != nil {
		t.Fatalf("migrate and seed must not need authentication settings: %v", err)
	}
}
