package hub

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sign returns the compact JSON of the envelope of the statement signed by the key, as the Hub signs the releases.
func sign(t *testing.T, key ed25519.PrivateKey, st *Statement) string {
	t.Helper()
	payload, err := json.Marshal(st)
	require.NoError(t, err)
	env := Envelope{PayloadType: PayloadType, Payload: payload, Signatures: []Signature{{
		KeyID: KeyID(key.Public().(ed25519.PublicKey)), Sig: ed25519.Sign(key, PAE(PayloadType, payload)),
	}}}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return string(raw)
}

func newKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return key
}

// trust replaces the pinned keys for the test.
func trust(t *testing.T, pub ...ed25519.PublicKey) {
	t.Helper()
	saved := keys
	keys = NewKeyRing(pub...)
	t.Cleanup(func() { keys = saved })
}

func statementOf(kind, id, version, source string) *Statement {
	sum := sha256.Sum256([]byte(source))
	return &Statement{
		Issuer: URL, Kind: kind, ID: id, Version: version, Language: "javascript",
		SHA256: hex.EncodeToString(sum[:]), Size: int64(len(source)), Official: true, IssuedAt: time.Unix(1700000000, 0).UTC(),
	}
}

func TestPinnedKeys(t *testing.T) {
	require.Len(t, keys, len(publicKeys))
	require.Contains(t, keys, "214d5d7593e53343")
}

func TestPAE(t *testing.T) {
	// The test vector of the DSSE specification.
	require.Equal(t, "DSSEv1 29 http://example.com/HelloWorld 11 hello world", string(PAE("http://example.com/HelloWorld", []byte("hello world"))))
}

// The fixture is signed by Sayrud Hub, it pins the wire format the releases are verified with.
func TestHubProtocolFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/releases.json")
	require.NoError(t, err)
	var fixtures []struct {
		PublicKey string
		Source    string
		Statement Statement
		Envelope  json.RawMessage
	}
	require.NoError(t, json.Unmarshal(raw, &fixtures))
	require.NotEmpty(t, fixtures)
	for _, f := range fixtures {
		pub, err := base64.StdEncoding.DecodeString(f.PublicKey)
		require.NoError(t, err)
		trust(t, pub)

		e := Expect{Kind: f.Statement.Kind, ID: f.Statement.ID, Version: f.Statement.Version, Language: f.Statement.Language}
		st, err := VerifyRelease(string(f.Envelope), e, f.Source)
		require.NoError(t, err)
		require.Equal(t, f.Statement, *st)
		require.Equal(t, &f.Statement, Origin(string(f.Envelope)))
	}
}

func TestVerifyRelease(t *testing.T) {
	const source = `export const manifest = {id: "slack", version: "1.0.0"}; export function execute() {}`
	key := newKey(t)
	trust(t, key.Public().(ed25519.PublicKey))
	signature := sign(t, key, statementOf(KindWorkflowPlugin, "slack", "1.0.0", source))
	expect := Expect{Kind: KindWorkflowPlugin, ID: "slack", Version: "1.0.0", Language: "javascript"}

	st, err := VerifyRelease(signature, expect, source)
	require.NoError(t, err)
	require.True(t, st.Official)
	require.NotNil(t, Origin(signature))

	for _, tc := range []struct {
		name   string
		expect Expect
		source string
		err    error
	}{
		{name: "modified source", expect: expect, source: source + " ", err: ErrStatementMismatch},
		{name: "another kind", expect: Expect{Kind: KindFieldShortcut, ID: "slack"}, source: source, err: ErrStatementMismatch},
		{name: "another id", expect: Expect{Kind: KindWorkflowPlugin, ID: "teams"}, source: source, err: ErrStatementMismatch},
		{name: "another version", expect: Expect{Kind: KindWorkflowPlugin, ID: "slack", Version: "1.0.1"}, source: source, err: ErrStatementMismatch},
		{name: "another language", expect: Expect{Kind: KindWorkflowPlugin, ID: "slack", Language: "typescript"}, source: source, err: ErrStatementMismatch},
		{name: "no id", expect: Expect{Kind: KindWorkflowPlugin}, source: source, err: ErrStatementMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyRelease(signature, tc.expect, tc.source)
			require.ErrorIs(t, err, tc.err)
		})
	}

	t.Run("untrusted key", func(t *testing.T) {
		forged := sign(t, newKey(t), statementOf(KindWorkflowPlugin, "slack", "1.0.0", source))
		_, err := VerifyRelease(forged, expect, source)
		require.ErrorIs(t, err, ErrUntrusted)
		require.Nil(t, Origin(forged))
	})

	t.Run("tampered payload", func(t *testing.T) {
		var env Envelope
		require.NoError(t, json.Unmarshal([]byte(signature), &env))
		st := statementOf(KindWorkflowPlugin, "slack", "1.0.0", source)
		st.Official = false
		env.Payload, _ = json.Marshal(st)
		raw, _ := json.Marshal(env)
		_, err := VerifyRelease(string(raw), expect, source)
		require.ErrorIs(t, err, ErrUntrusted)
	})

	t.Run("another issuer", func(t *testing.T) {
		st := statementOf(KindWorkflowPlugin, "slack", "1.0.0", source)
		st.Issuer = "https://hub.example.com"
		other := sign(t, key, st)
		_, err := VerifyRelease(other, expect, source)
		require.ErrorIs(t, err, ErrStatementMismatch)
		require.Nil(t, Origin(other))
	})

	t.Run("not an envelope", func(t *testing.T) {
		_, err := VerifyRelease("", expect, source)
		require.ErrorIs(t, err, ErrUnsupportedPayloadType)
		require.Nil(t, Origin("{"))
		require.Nil(t, Origin(""))
	})
}

func TestParseSourceURL(t *testing.T) {
	for _, tc := range []struct {
		url               string
		kind, id, version string
		ok                bool
	}{
		{url: URL + "/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js", kind: KindWorkflowPlugin, id: "slack", version: "1.2.0", ok: true},
		{url: URL + "/open/v1/packages/field-shortcut/translate/versions/latest/source.ts", kind: KindFieldShortcut, id: "translate", version: "latest", ok: true},
		{url: " " + URL + "/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js ", kind: KindWorkflowPlugin, id: "slack", version: "1.2.0", ok: true},
		{url: URL + "/open/v1/packages/workflow_plugin/slack/versions/1.2.0/source.js"},
		{url: URL + "/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.py"},
		{url: URL + "/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js?x=1"},
		{url: URL + "/open/v1/packages/workflow-plugin/slack"},
		{url: "https://hub.sayrud.com.evil.com/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js"},
		{url: "http://hub.sayrud.com/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js"},
		{url: "https://example.com/plugin.js"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			kind, id, version, ok := ParseSourceURL(tc.url)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.kind, kind)
			require.Equal(t, tc.id, id)
			require.Equal(t, tc.version, version)
		})
	}
}

func TestPageURL(t *testing.T) {
	require.Equal(t, "https://hub.sayrud.com/packages/workflow-plugin/slack", PageURL(KindWorkflowPlugin, "slack"))
	require.Equal(t, "https://hub.sayrud.com/packages/field-shortcut/translate", PageURL(KindFieldShortcut, "translate"))
}
