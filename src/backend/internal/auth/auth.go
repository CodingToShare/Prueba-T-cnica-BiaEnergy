// Package auth is the minimal demo authentication of the challenge (OD-12):
// one configured credential and a signed session cookie. It is deliberately
// not an identity system: there are no user records, roles, registration,
// password storage or server-side session store.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CookieName is the session cookie.
const CookieName = "bia_session"

// SessionTTL is how long a session stays valid after login.
const SessionTTL = 8 * time.Hour

// Minimum lengths accepted for the configured secrets.
const (
	MinPasswordLength   = 8
	MinSigningKeyLength = 32
)

// Errors returned by Verify. Callers answer both with the same 401.
var (
	ErrInvalidSession = errors.New("invalid session")
	ErrExpiredSession = errors.New("expired session")
)

// Config is the demo credential and cookie policy.
type Config struct {
	Username string
	Password string
	// SigningKey signs session cookies with HMAC-SHA256.
	SigningKey []byte
	// SecureCookie sets the cookie's Secure attribute. It is true unless
	// explicitly disabled for local HTTP development.
	SecureCookie bool
}

// Validate reports every missing or unsafe setting without echoing values.
func (c Config) Validate() error {
	var problems []error
	if strings.TrimSpace(c.Username) == "" || strings.TrimSpace(c.Username) != c.Username {
		problems = append(problems, errors.New("demo username is required and must not have surrounding whitespace"))
	}
	if len(c.Password) < MinPasswordLength {
		problems = append(problems, fmt.Errorf("demo password must have at least %d characters", MinPasswordLength))
	}
	if len(c.SigningKey) < MinSigningKeyLength {
		problems = append(problems, fmt.Errorf("session signing key must have at least %d bytes", MinSigningKeyLength))
	}
	return errors.Join(problems...)
}

// Session is the content of a valid session cookie. The payload is signed,
// not encrypted: it holds nothing secret.
type Session struct {
	Subject   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type payload struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

// Manager checks credentials and issues and verifies session tokens.
type Manager struct {
	cfg Config
	now func() time.Time
}

// NewManager validates the configuration. now is injectable for tests; nil
// means time.Now.
func NewManager(cfg Config, now func() time.Time) (*Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid authentication configuration: %w", err)
	}
	if now == nil {
		now = time.Now
	}
	cfg.SigningKey = append([]byte(nil), cfg.SigningKey...)
	return &Manager{cfg: cfg, now: now}, nil
}

// CheckCredentials compares both fields in constant time (over their
// SHA-256 digests, so lengths do not leak either) and always evaluates both.
func (m *Manager) CheckCredentials(username, password string) bool {
	userOK := equalDigest(username, m.cfg.Username)
	passOK := equalDigest(password, m.cfg.Password)
	return userOK&passOK == 1
}

func equalDigest(a, b string) int {
	da, db := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(da[:], db[:])
}

// Issue creates a new signed session token for the configured user.
func (m *Manager) Issue() (Session, string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return Session{}, "", fmt.Errorf("generate session nonce: %w", err)
	}
	now := m.now().UTC().Truncate(time.Second)
	s := Session{Subject: m.cfg.Username, IssuedAt: now, ExpiresAt: now.Add(SessionTTL)}
	body, err := json.Marshal(payload{
		Subject: s.Subject, IssuedAt: s.IssuedAt.Unix(), ExpiresAt: s.ExpiresAt.Unix(),
		Nonce: hex.EncodeToString(nonce),
	})
	if err != nil {
		return Session{}, "", fmt.Errorf("encode session: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	return s, encoded + "." + base64.RawURLEncoding.EncodeToString(m.sign(encoded)), nil
}

func (m *Manager) sign(encodedPayload string) []byte {
	mac := hmac.New(sha256.New, m.cfg.SigningKey)
	mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}

// Verify checks the signature first, then the claims: the subject must be
// the configured user, the session must not be expired, and its lifetime
// must not exceed SessionTTL.
func (m *Manager) Verify(token string) (Session, error) {
	encoded, sig, ok := strings.Cut(token, ".")
	if !ok || encoded == "" || strings.Contains(sig, ".") {
		return Session{}, ErrInvalidSession
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(given, m.sign(encoded)) {
		return Session{}, ErrInvalidSession
	}
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	var p payload
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Session{}, ErrInvalidSession
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Session{}, ErrInvalidSession
	}
	if equalDigest(p.Subject, m.cfg.Username) != 1 || p.ExpiresAt <= p.IssuedAt ||
		uint64(p.ExpiresAt)-uint64(p.IssuedAt) > uint64(SessionTTL/time.Second) {
		return Session{}, ErrInvalidSession
	}
	s := Session{Subject: p.Subject, IssuedAt: time.Unix(p.IssuedAt, 0).UTC(), ExpiresAt: time.Unix(p.ExpiresAt, 0).UTC()}
	now := m.now()
	if !now.Before(s.ExpiresAt) {
		return Session{}, ErrExpiredSession
	}
	if s.IssuedAt.After(now.Add(time.Minute)) {
		return Session{}, ErrInvalidSession
	}
	return s, nil
}

// FromRequest verifies the session cookie of a request.
func (m *Manager) FromRequest(r *http.Request) (Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return Session{}, ErrInvalidSession
	}
	return m.Verify(c.Value)
}

// SessionCookie carries a token: HttpOnly, SameSite=Lax, Path=/, and Secure
// unless disabled by configuration.
func (m *Manager) SessionCookie(token string, s Session) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  s.ExpiresAt,
		MaxAge:   int(SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   m.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearCookie removes the session cookie in the browser.
func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
	}
}
