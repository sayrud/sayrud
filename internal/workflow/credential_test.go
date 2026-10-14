package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareCredential(t *testing.T) {
	def := CredentialType("smtp")
	require.NotNil(t, def)

	data, secrets, err := PrepareCredential(def, CredentialInput{
		Data:    map[string]interface{}{"host": " smtp.example.com ", "fromAddress": "bot@example.com", "password": "leak", "extra": "x"},
		Secrets: map[string]string{"password": "p1"},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"host": "smtp.example.com", "fromAddress": "bot@example.com", "security": "tls", "port": 465.0}, data,
		"defaults are filled in, secret and unknown fields never stay in the plain data")
	assert.Equal(t, map[string]string{"password": "p1"}, secrets)

	_, secrets, err = PrepareCredential(def, CredentialInput{Data: data, Secrets: map[string]string{"password": ""}}, secrets)
	require.NoError(t, err)
	assert.Equal(t, "p1", secrets["password"], "an empty secret keeps the saved one")

	_, secrets, err = PrepareCredential(def, CredentialInput{Data: data, ClearSecrets: []string{"password"}}, secrets)
	require.NoError(t, err)
	assert.Empty(t, secrets)

	for _, bad := range []map[string]interface{}{
		{"fromAddress": "bot@example.com"},
		{"host": "smtp.example.com", "fromAddress": "not an email"},
		{"host": "smtp.example.com", "fromAddress": "bot@example.com", "port": 70000.0},
		{"host": "smtp.example.com", "fromAddress": "bot@example.com", "security": "ssl"},
	} {
		_, _, err := PrepareCredential(def, CredentialInput{Data: bad}, nil)
		assert.Error(t, err, bad)
	}
}
