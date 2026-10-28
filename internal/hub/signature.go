package hub

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The verification below follows the signing of Sayrud Hub, testdata/releases.json pins its wire format.

// PayloadType is the DSSE payload type of a release statement, the statements of any other type are refused.
const PayloadType = "application/vnd.sayrud.hub.release.v1+json"

// Kinds of the packages.
const (
	KindWorkflowPlugin = "workflow_plugin"
	KindFieldShortcut  = "field_shortcut"
)

// maxSignatures bounds the verification work of an envelope.
const maxSignatures = 8

// Statement is what the Hub asserts about an approved release.
type Statement struct {
	// Issuer is the base URL of the Hub that approved the release.
	Issuer string `json:"issuer"`
	// Kind is workflow_plugin or field_shortcut, a source must only be installed as the kind it was approved as.
	Kind string `json:"kind"`
	// ID is the id of the package, which equals the id exported by the manifest of the source.
	ID string `json:"id"`
	// Version is the version of the release, which equals the version exported by the manifest of the source.
	Version string `json:"version"`
	// Language is the source language, javascript or typescript.
	Language string `json:"language"`
	// SHA256 is the lowercase hex SHA-256 of the source bytes.
	SHA256 string `json:"sha256"`
	// Size is the length of the source in bytes.
	Size int64 `json:"size"`
	// Official reports whether the package is published by the Sayrud team.
	Official bool `json:"official"`
	// IssuedAt is the time the statement was signed.
	IssuedAt time.Time `json:"issuedAt"`
}

// Envelope is a DSSE envelope, the byte slices are encoded in standard base64 in JSON.
type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     []byte      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

// Signature is a signature of an envelope.
type Signature struct {
	// KeyID identifies the public key, see KeyID.
	KeyID string `json:"keyid"`
	Sig   []byte `json:"sig"`
}

var (
	ErrUnsupportedPayloadType = errors.New("signature: unsupported payload type")
	ErrNoSignature            = errors.New("signature: the envelope is not signed")
	ErrTooManySignatures      = errors.New("signature: the envelope has too many signatures")
	// ErrUntrusted means none of the signatures is made by a trusted key, or none of them is valid.
	ErrUntrusted         = errors.New("signature: no valid signature of a trusted key")
	ErrInvalidStatement  = errors.New("signature: invalid statement")
	ErrStatementMismatch = errors.New("signature: the statement does not match the release")
)

// PAE returns the DSSE pre-authentication encoding of the payload, which is the message actually signed.
func PAE(payloadType string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString("DSSEv1 ")
	b.WriteString(strconv.Itoa(len(payloadType)))
	b.WriteByte(' ')
	b.WriteString(payloadType)
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(payload)))
	b.WriteByte(' ')
	b.Write(payload)
	return b.Bytes()
}

// KeyID returns the identifier of the public key, the hex of the first 8 bytes of its SHA-256.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

// KeyRing is the set of the trusted public keys keyed by KeyID.
type KeyRing map[string]ed25519.PublicKey

// NewKeyRing returns the key ring of the public keys.
func NewKeyRing(keys ...ed25519.PublicKey) KeyRing {
	ring := make(KeyRing, len(keys))
	for _, k := range keys {
		ring[KeyID(k)] = k
	}
	return ring
}

// Verify checks that at least one signature of the envelope is made by a trusted key, and returns the signed statement.
// The statement still has to be checked against the release being installed, see VerifySource.
func Verify(env *Envelope, keys KeyRing) (*Statement, error) {
	if env == nil || env.PayloadType != PayloadType {
		return nil, ErrUnsupportedPayloadType
	}
	switch {
	case len(env.Signatures) == 0:
		return nil, ErrNoSignature
	case len(env.Signatures) > maxSignatures:
		return nil, ErrTooManySignatures
	}

	message := PAE(env.PayloadType, env.Payload)
	trusted := false
	for _, sig := range env.Signatures {
		key, ok := keys[sig.KeyID]
		if ok && len(key) == ed25519.PublicKeySize && len(sig.Sig) == ed25519.SignatureSize && ed25519.Verify(key, message, sig.Sig) {
			trusted = true
			break
		}
	}
	if !trusted {
		return nil, ErrUntrusted
	}

	// Unknown fields are ignored on purpose: a field restricting the use of a release comes with a new payload type.
	var st Statement
	if err := json.Unmarshal(env.Payload, &st); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStatement, err)
	}
	if err := st.validate(); err != nil {
		return nil, err
	}
	return &st, nil
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (st *Statement) validate() error {
	switch {
	case st.Issuer == "", st.ID == "", st.Version == "", st.Language == "":
		return fmt.Errorf("%w: missing issuer, id, version or language", ErrInvalidStatement)
	case st.Kind != KindWorkflowPlugin && st.Kind != KindFieldShortcut:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidStatement, st.Kind)
	case !sha256Pattern.MatchString(st.SHA256):
		return fmt.Errorf("%w: sha256 must be 64 lowercase hex characters", ErrInvalidStatement)
	case st.Size < 0:
		return fmt.Errorf("%w: negative size", ErrInvalidStatement)
	}
	return nil
}

// Expect is the release the verifier intends to install.
type Expect struct {
	// Issuer is the base URL of the Hub, required.
	Issuer string
	// Kind is the kind the source is installed as, required.
	Kind string
	// ID is the id of the package, required.
	ID string
	// Version is the version of the release, it is not checked if empty.
	Version string
	// Language is the source language, it is not checked if empty.
	Language string
}

// Check reports whether the statement describes exactly the source of the expected release.
func (st *Statement) Check(e Expect, source []byte) error {
	if e.Issuer == "" || e.Kind == "" || e.ID == "" {
		return fmt.Errorf("%w: the issuer, kind and id to expect are required", ErrStatementMismatch)
	}
	sum := sha256.Sum256(source)
	switch {
	case strings.TrimSuffix(st.Issuer, "/") != strings.TrimSuffix(e.Issuer, "/"):
		return fmt.Errorf("%w: issued by %q", ErrStatementMismatch, st.Issuer)
	case st.Kind != e.Kind:
		return fmt.Errorf("%w: approved as a %s", ErrStatementMismatch, st.Kind)
	case st.ID != e.ID:
		return fmt.Errorf("%w: approved as %q", ErrStatementMismatch, st.ID)
	case e.Version != "" && st.Version != e.Version:
		return fmt.Errorf("%w: approved as version %q", ErrStatementMismatch, st.Version)
	case e.Language != "" && st.Language != e.Language:
		return fmt.Errorf("%w: approved as %s", ErrStatementMismatch, st.Language)
	case st.Size != int64(len(source)) || st.SHA256 != hex.EncodeToString(sum[:]):
		return fmt.Errorf("%w: the source has been modified", ErrStatementMismatch)
	}
	return nil
}

// VerifySource verifies the envelope with the trusted keys and checks that it describes the source of the expected release.
func VerifySource(env *Envelope, keys KeyRing, e Expect, source []byte) (*Statement, error) {
	st, err := Verify(env, keys)
	if err != nil {
		return nil, err
	}
	if err := st.Check(e, source); err != nil {
		return nil, err
	}
	return st, nil
}
