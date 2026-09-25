package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerify_AuditLifetimeAndDocumentBoundaries(t *testing.T) {
	m, c := newTestManager(t)
	now := c.t.Unix()
	for _, tc := range []struct {
		name     string
		iat, exp int64
		trailer  string
		valid    bool
	}{
		{"exact maximum lifetime", now, now + 28800, "", true},
		{"duration overflow", now, now + 18446744074, "", false},
		{"signed subtraction overflow", -9223372036854775808, now + 28800, "", false},
		{"trailing document", now, now + 28800, " {}", false},
		{"clock tolerance boundary", now + 60, now + 28800, "", true},
		{"beyond clock tolerance", now + 61, now + 28800, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(payload{Subject: testConfig.Username, IssuedAt: tc.iat, ExpiresAt: tc.exp, Nonce: "00112233445566778899aabbccddeeff"})
			require.NoError(t, err)
			encoded := base64.RawURLEncoding.EncodeToString(append(body, tc.trailer...))
			_, err = m.Verify(encoded + "." + base64.RawURLEncoding.EncodeToString(m.sign(encoded)))
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalidSession)
			}
		})
	}
}
