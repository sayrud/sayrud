// Package sso implements the third-party sign-in with OAuth 2.0, OpenID Connect, SAML 2.0 and LDAP.
package sso

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/conf"
)

var (
	keyMu       sync.RWMutex
	keyOverride []byte
)

// SetKey overrides the key of the config file for tests, nil restores reading the config file.
func SetKey(key []byte) {
	keyMu.Lock()
	defer keyMu.Unlock()
	keyOverride = key
}

func secretKey() ([]byte, error) {
	keyMu.RLock()
	override := keyOverride
	keyMu.RUnlock()
	if override != nil {
		return override, nil
	}
	raw := strings.TrimSpace(conf.Auth.SecretKey)
	if raw == "" {
		return nil, ErrNoSecretKey
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidSecretKey
	}
	return key, nil
}

var (
	ErrNoSecretKey      = errors.New("auth.secret_key is not configured")
	ErrInvalidSecretKey = errors.New("auth.secret_key must be the base64 of 32 bytes")
)

// CheckSecretKey validates the key on startup, it returns nil if not configured and ErrInvalidSecretKey if malformed.
func CheckSecretKey() error {
	if _, err := secretKey(); err != nil && !errors.Is(err, ErrNoSecretKey) {
		return err
	}
	return nil
}

// SecretsReady reports whether a valid key is configured.
func SecretsReady() bool {
	_, err := secretKey()
	return err == nil
}

// Seal encrypts with AES-256-GCM and returns base64(nonce || ciphertext).
func Seal(plaintext []byte) (string, error) {
	aead, err := newAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", errors.Wrap(err, "generate nonce")
	}
	return base64.StdEncoding.EncodeToString(aead.Seal(nonce, nonce, plaintext, nil)), nil
}

// Open decrypts the result of Seal.
func Open(sealed string) ([]byte, error) {
	aead, err := newAEAD()
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return nil, errors.Wrap(err, "decode base64")
	}
	if len(raw) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	plaintext, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	if err != nil {
		return nil, errors.Wrap(err, "decrypt")
	}
	return plaintext, nil
}

func newAEAD() (cipher.AEAD, error) {
	key, err := secretKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "new cipher")
	}
	return cipher.NewGCM(block)
}

// Secrets are the sensitive config of a provider, stored encrypted in auth_providers.secrets.
type Secrets struct {
	ClientSecret string `json:"clientSecret,omitempty"`
	BindPassword string `json:"bindPassword,omitempty"`
	SPPrivateKey string `json:"spPrivateKey,omitempty"`
}

// IsZero reports whether there is no secret.
func (s Secrets) IsZero() bool {
	return s == Secrets{}
}

// EncodeSecrets encrypts the secrets, it returns empty if all of them are empty.
func EncodeSecrets(s Secrets) (string, error) {
	if s.IsZero() {
		return "", nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return "", errors.Wrap(err, "marshal")
	}
	return Seal(raw)
}

// DecodeSecrets decrypts the result of EncodeSecrets, empty returns the zero value.
func DecodeSecrets(sealed string) (Secrets, error) {
	var s Secrets
	if sealed == "" {
		return s, nil
	}
	raw, err := Open(sealed)
	if err != nil {
		return s, err
	}
	return s, errors.Wrap(json.Unmarshal(raw, &s), "unmarshal")
}
