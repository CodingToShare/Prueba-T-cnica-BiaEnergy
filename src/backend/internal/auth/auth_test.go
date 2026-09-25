package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-only credentials; never used outside tests.
var testConfig = Config{
	Username:   "test-operator",
	Password:   "test-password-123",
	SigningKey: []byte("test-signing-key-0123456789abcdef-test"),
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newTestManager(t *testing.T) (*Manager, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)}
	m, err := NewManager(testConfig, c.now)
	require.NoError(t, err)
	return m, c
}

func TestConfig_ValidateRejectsMissingOrWeakSettingsWithoutEchoingThem(t *testing.T) {
	err := Config{Username: " x", Password: "short", SigningKey: []byte("tiny-secret")}.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "username")
	assert.Contains(t, err.Error(), "at least 8 characters")
	assert.Contains(t, err.Error(), "at least 32 bytes")
	assert.NotContains(t, err.Error(), "short")
	assert.NotContains(t, err.Error(), "tiny-secret")
	assert.NoError(t, testConfig.Validate())
}

func TestCheckCredentials(t *testing.T) {
	m, _ := newTestManager(t)

	assert.True(t, m.CheckCredentials("test-operator", "test-password-123"))
	assert.False(t, m.CheckCredentials("test-operator", "wrong"))
	assert.False(t, m.CheckCredentials("someone-else", "test-password-123"))
	assert.False(t, m.CheckCredentials("", ""))
	assert.False(t, m.CheckCredentials("test-operator", "test-password-1234"), "a prefix match is not enough")
}

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	m, c := newTestManager(t)

	s, token, err := m.Issue()
	require.NoError(t, err)
	assert.Equal(t, "test-operator", s.Subject)
	assert.Equal(t, c.t, s.IssuedAt)
	assert.Equal(t, c.t.Add(SessionTTL), s.ExpiresAt)
	assert.NotContains(t, token, "test-password-123")

	got, err := m.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, s, got)

	_, other, err := m.Issue()
	require.NoError(t, err)
	assert.NotEqual(t, token, other, "a random nonce makes every token distinct")
}

func TestVerify_Expiration(t *testing.T) {
	m, c := newTestManager(t)
	_, token, err := m.Issue()
	require.NoError(t, err)

	c.t = c.t.Add(SessionTTL - time.Second)
	_, err = m.Verify(token)
	assert.NoError(t, err, "valid until the last second")

	c.t = c.t.Add(time.Second)
	_, err = m.Verify(token)
	assert.ErrorIs(t, err, ErrExpiredSession)
}

func TestVerify_RejectsTamperedAndMalformedTokens(t *testing.T) {
	m, _ := newTestManager(t)
	_, token, err := m.Issue()
	require.NoError(t, err)
	payloadPart, sigPart, _ := strings.Cut(token, ".")

	forged := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"test-operator","iat":1,"exp":99999999999,"nonce":"x"}`))
	otherKey, err := NewManager(Config{Username: testConfig.Username, Password: testConfig.Password, SigningKey: []byte("another-signing-key-0123456789abcdef")}, nil)
	require.NoError(t, err)
	_, foreign, err := otherKey.Issue()
	require.NoError(t, err)

	for name, bad := range map[string]string{
		"empty":                 "",
		"no separator":          payloadPart,
		"extra part":            token + ".x",
		"payload swapped":       forged + "." + sigPart,
		"signature altered":     payloadPart + "." + strings.Repeat("A", len(sigPart)),
		"signature not base64":  payloadPart + ".!!!",
		"signed by another key": foreign,
		"missing payload":       "." + sigPart,
	} {
		_, err := m.Verify(bad)
		assert.ErrorIs(t, err, ErrInvalidSession, name)
	}
}

func TestVerify_RejectsValidlySignedButImpossibleClaims(t *testing.T) {
	m, c := newTestManager(t)
	sign := func(payloadJSON string) string {
		enc := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
		return enc + "." + base64.RawURLEncoding.EncodeToString(m.sign(enc))
	}
	now := c.t.Unix()
	for name, p := range map[string]string{
		"other subject":        `{"sub":"admin","iat":` + itoa(now) + `,"exp":` + itoa(now+60) + `,"nonce":"n"}`,
		"lifetime beyond TTL":  `{"sub":"test-operator","iat":` + itoa(now) + `,"exp":` + itoa(now+int64(SessionTTL/time.Second)+1) + `,"nonce":"n"}`,
		"issued in the future": `{"sub":"test-operator","iat":` + itoa(now+3600) + `,"exp":` + itoa(now+7200) + `,"nonce":"n"}`,
		"unknown field":        `{"sub":"test-operator","iat":` + itoa(now) + `,"exp":` + itoa(now+60) + `,"nonce":"n","role":"admin"}`,
		"not JSON":             `not-json`,
	} {
		_, err := m.Verify(sign(p))
		assert.ErrorIs(t, err, ErrInvalidSession, name)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func TestCookies_HaveTheRequiredAttributes(t *testing.T) {
	m, _ := newTestManager(t)
	s, token, err := m.Issue()
	require.NoError(t, err)

	c := m.SessionCookie(token, s)
	assert.Equal(t, CookieName, c.Name)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, "/", c.Path)
	assert.False(t, c.Secure, "Secure follows the configuration (disabled here)")
	assert.Equal(t, int(SessionTTL/time.Second), c.MaxAge)

	secure, err := NewManager(Config{Username: testConfig.Username, Password: testConfig.Password, SigningKey: testConfig.SigningKey, SecureCookie: true}, nil)
	require.NoError(t, err)
	assert.True(t, secure.SessionCookie(token, s).Secure)

	clear := m.ClearCookie()
	assert.Equal(t, CookieName, clear.Name)
	assert.Empty(t, clear.Value)
	assert.Negative(t, clear.MaxAge)
	assert.True(t, clear.HttpOnly)
}

func TestFromRequest(t *testing.T) {
	m, _ := newTestManager(t)
	s, token, err := m.Issue()
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err = m.FromRequest(r)
	assert.ErrorIs(t, err, ErrInvalidSession, "no cookie")

	r.AddCookie(m.SessionCookie(token, s))
	got, err := m.FromRequest(r)
	require.NoError(t, err)
	assert.Equal(t, "test-operator", got.Subject)
}
