// Package hub connects Sayrud to Sayrud Hub, the catalog of the workflow plugins and field shortcuts reviewed by the
// Sayrud team. Every approved release is signed by the Hub; Sayrud pins the Hub URL and its public keys, verifies the
// sources it downloads and only records a package as installed from the Hub when its signature describes the exact
// installed source.
package hub

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// URL is the base URL of Sayrud Hub, which is also the issuer the release statements must carry.
const URL = "https://hub.sayrud.com"

// publicKeys are the standard base64 Ed25519 public keys trusted to sign the releases.
// To rotate, ship a release pinning both keys, then drop the old key once the Hub no longer signs with it.
var publicKeys = []string{
	"66lO7o+RNf8MR+QtMGTuC/A+WXc2F6tPitq31kO/kPM=", // 214d5d7593e53343
}

var (
	// baseURL is where the API is requested, the tests point it to a local server while the issuer stays URL.
	baseURL = URL
	keys    = mustKeyRing(publicKeys)
)

func mustKeyRing(encoded []string) KeyRing {
	list := make([]ed25519.PublicKey, 0, len(encoded))
	for _, s := range encoded {
		raw, err := base64.StdEncoding.DecodeString(s)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			panic(fmt.Sprintf("hub: invalid pinned public key %q", s))
		}
		list = append(list, raw)
	}
	return NewKeyRing(list...)
}

// LatestVersion is the version path parameter of the latest release.
const LatestVersion = "latest"

// kindPath returns the kind as written in the URL paths of the Hub, e.g. workflow-plugin.
func kindPath(kind string) string {
	return strings.ReplaceAll(kind, "_", "-")
}

// PageURL returns the page of the package on Sayrud Hub.
func PageURL(kind, id string) string {
	return URL + "/packages/" + kindPath(kind) + "/" + url.PathEscape(id)
}

// ParseSourceURL reports whether the URL downloads the source of a release from the Hub, and returns its kind, id and version.
func ParseSourceURL(raw string) (kind, id, version string, ok bool) {
	prefix := baseURL + "/open/v1/packages/"
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.String(), prefix) {
		return "", "", "", false
	}
	// {kind}/{id}/versions/{version}/source.{js,ts}
	parts := strings.Split(strings.TrimPrefix(u.String(), prefix), "/")
	if len(parts) != 5 || parts[2] != "versions" || (parts[4] != "source.js" && parts[4] != "source.ts") {
		return "", "", "", false
	}
	switch parts[0] {
	case kindPath(KindWorkflowPlugin):
		kind = KindWorkflowPlugin
	case kindPath(KindFieldShortcut):
		kind = KindFieldShortcut
	default:
		return "", "", "", false
	}
	if parts[1] == "" || parts[3] == "" {
		return "", "", "", false
	}
	return kind, parts[1], parts[3], true
}

// decodeEnvelope decodes the JSON of a DSSE envelope.
func decodeEnvelope(signature string) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal([]byte(signature), &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedPayloadType, err)
	}
	return &env, nil
}

// VerifyRelease verifies the JSON of the envelope with the pinned keys and checks that it describes the source of the
// expected release issued by the Hub; Expect.Issuer is ignored.
func VerifyRelease(signature string, e Expect, source string) (*Statement, error) {
	env, err := decodeEnvelope(signature)
	if err != nil {
		return nil, err
	}
	e.Issuer = URL
	return VerifySource(env, keys, e, []byte(source))
}

// Origin returns the statement of the stored envelope of an installed package, nil if it is empty or no longer
// trusted. The source is not checked, as it was when the package was saved.
func Origin(signature string) *Statement {
	if signature == "" {
		return nil
	}
	env, err := decodeEnvelope(signature)
	if err != nil {
		return nil
	}
	st, err := Verify(env, keys)
	if err != nil || strings.TrimSuffix(st.Issuer, "/") != URL {
		return nil
	}
	return st
}
