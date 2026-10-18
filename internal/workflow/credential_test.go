package workflow

import (
	"encoding/json"
	"strings"
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

func TestGCPServiceAccountCredential(t *testing.T) {
	def := CredentialType("cloud")
	require.NotNil(t, def)

	account := map[string]interface{}{
		"type": "service_account", "project_id": "demo", "client_email": "bot@demo.iam.gserviceaccount.com",
		"private_key": "-----BEGIN PRIVATE KEY-----\n" + strings.Repeat("key-material", 200) + "\n-----END PRIVATE KEY-----\n",
	}
	raw, err := json.MarshalIndent(account, "", "  ")
	require.NoError(t, err)

	data, secrets, err := PrepareCredential(def, CredentialInput{Data: map[string]interface{}{"provider": "gcp"}, Secrets: map[string]string{"serviceAccountJson": string(raw)}}, nil)
	require.NoError(t, err, "multiline JSON keys longer than the default 255 characters are supported")
	require.Equal(t, map[string]interface{}{"provider": "gcp"}, data)
	require.Equal(t, string(raw), secrets["serviceAccountJson"])

	for _, bad := range []string{"not-json", "null", "[]", "{}", `{"type":"authorized_user"}`, strings.Repeat("x", 32769)} {
		_, _, err := PrepareCredential(def, CredentialInput{Data: map[string]interface{}{"provider": "gcp"}, Secrets: map[string]string{"serviceAccountJson": bad}}, nil)
		require.Error(t, err, bad[:min(len(bad), 50)])
	}

	for _, key := range []string{"type", "project_id", "client_email", "private_key"} {
		t.Run(key, func(t *testing.T) {
			invalid := map[string]interface{}{}
			for k, v := range account {
				invalid[k] = v
			}

			invalid[key] = " "
			raw, err := json.Marshal(invalid)
			require.NoError(t, err)

			_, _, err = PrepareCredential(def, CredentialInput{Data: map[string]interface{}{"provider": "gcp"}, Secrets: map[string]string{"serviceAccountJson": string(raw)}}, nil)
			require.Error(t, err)
		})
	}
}

func TestCloudCredentialProviderSwitch(t *testing.T) {
	def := CredentialType("cloud")
	require.NotNil(t, def)

	current := map[string]string{"secretAccessKey": "aws-secret", "clientSecret": "stale-secret"}
	input := CredentialInput{
		Data:        map[string]interface{}{"provider": "volcengine", "accessKeyId": "new-id", "tenantId": "stale-tenant"},
		CurrentData: []byte(`{"provider":"aws","accessKeyId":"old-id"}`),
	}

	_, _, err := PrepareCredential(def, input, current)
	require.Error(t, err, "switching providers requires a new secret, even when they use the same field name")

	input.Secrets = map[string]string{"secretAccessKey": "volcengine-secret", "clientSecret": "ignored-secret"}
	data, secrets, err := PrepareCredential(def, input, current)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"provider": "volcengine", "accessKeyId": "new-id"}, data)
	require.Equal(t, map[string]string{"secretAccessKey": "volcengine-secret"}, secrets)

	input.CurrentData = []byte(`{"provider":"volcengine"}`)
	input.Secrets = nil
	_, secrets, err = PrepareCredential(def, input, current)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"secretAccessKey": "aws-secret"}, secrets, "unchanged providers preserve only active secrets")

	for _, provider := range []interface{}{nil, "", "unknown", 42.0, map[string]interface{}{"value": "aws"}} {
		input.Data["provider"] = provider
		_, _, err := PrepareCredential(def, input, current)
		require.Error(t, err, "missing or invalid providers are rejected")
	}
}
