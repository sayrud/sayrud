// Package setup implements the installation wizard served before Sayrud is configured and has an admin.
package setup

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"
)

// TokenEnv overrides the generated setup token, so that all the replicas accept the same one.
const TokenEnv = "SAYRUD_SETUP_TOKEN"

const (
	tokenBytes     = 15
	tokenGroupSize = 4
	// minTokenLength is the minimum normalized length of the tokens provided by TokenEnv.
	minTokenLength = 16
)

// Token is the one-time secret proving that the installer can read the server log or environment.
type Token struct {
	digest  [sha256.Size]byte
	display string
}

// NewToken generates a random token formatted as dash-separated groups, e.g. ABCD-EFGH-IJKL-MNOP-QRST-UVWX.
func NewToken() (*Token, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, errors.Wrap(err, "read random bytes")
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)

	var b strings.Builder
	for i, r := range encoded {
		if i > 0 && i%tokenGroupSize == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return newToken(b.String()), nil
}

// ParseToken accepts the token provided by the operator, it must have at least 16 characters besides dashes and spaces.
func ParseToken(value string) (*Token, error) {
	if len(normalizeToken(value)) < minTokenLength {
		return nil, errors.Newf("setup token must have at least %d characters", minTokenLength)
	}
	return newToken(strings.TrimSpace(value)), nil
}

func newToken(display string) *Token {
	return &Token{digest: sha256.Sum256([]byte(normalizeToken(display))), display: display}
}

// String returns the token as it is shown to the operator.
func (t *Token) String() string {
	return t.display
}

// Verify reports whether the input matches the token, ignoring case, spaces and dashes, in constant time.
func (t *Token) Verify(input string) bool {
	digest := sha256.Sum256([]byte(normalizeToken(input)))
	return subtle.ConstantTimeCompare(digest[:], t.digest[:]) == 1
}

func normalizeToken(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, value)
}
