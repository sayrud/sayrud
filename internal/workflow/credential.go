package workflow

import (
	"context"
	"encoding/json"
	"math"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/sso"
)

const defaultCredentialMaxLength = 255

// CredentialInput is a submitted credential: empty Secrets keep the saved values, and the fields in ClearSecrets are cleared.
type CredentialInput struct {
	Data         map[string]interface{}
	Secrets      map[string]string
	ClearSecrets []string
}

// PrepareCredential validates and splits the fields by the credential type DSL, returning the plain data and the merged secrets; current are the saved secrets, empty on creation.
// Errors are *i18n.Error to be translated.
func PrepareCredential(def *CredentialTypeDef, in CredentialInput, current map[string]string) (map[string]interface{}, map[string]string, error) {
	data := map[string]interface{}{}
	secrets := map[string]string{}
	for k, v := range current {
		secrets[k] = v
	}
	for _, k := range in.ClearSecrets {
		delete(secrets, k)
	}
	for _, f := range def.Fields() {
		if f.Secret {
			if v := in.Secrets[f.Key]; v != "" {
				secrets[f.Key] = v
			}
			v := secrets[f.Key]
			if f.Required && v == "" {
				return nil, nil, i18n.Errorf("credential::field_required", f.Key)
			}
			if max := maxLength(f); utf8.RuneCountInString(v) > max {
				return nil, nil, i18n.Errorf("credential::too_long", f.Key, max)
			}
			continue
		}
		v, ok := in.Data[f.Key]
		if !ok || v == nil {
			if len(f.Default) > 0 {
				_ = json.Unmarshal(f.Default, &v)
			}
		}
		v, err := normalizeCredentialValue(f, v)
		if err != nil {
			return nil, nil, err
		}
		if v != nil {
			data[f.Key] = v
		}
	}
	for k := range secrets {
		if !isSecretField(def, k) {
			delete(secrets, k)
		}
	}
	return data, secrets, nil
}

func maxLength(f FieldDef) int {
	if f.MaxLength > 0 {
		return f.MaxLength
	}
	return defaultCredentialMaxLength
}

func isSecretField(def *CredentialTypeDef, key string) bool {
	for _, f := range def.Fields() {
		if f.Key == key {
			return f.Secret
		}
	}
	return false
}

// normalizeCredentialValue validates a non-secret field and converts it to its canonical type, nil for empty values.
func normalizeCredentialValue(f FieldDef, v interface{}) (interface{}, error) {
	switch f.Widget {
	case "number":
		n, ok := v.(float64)
		if !ok || n == 0 {
			if f.Required {
				return nil, i18n.Errorf("credential::field_required", f.Key)
			}
			return nil, nil
		}
		if n != math.Trunc(n) || (f.Min != nil && n < *f.Min) || (f.Max != nil && n > *f.Max) {
			return nil, i18n.Errorf("credential::invalid_field", f.Key)
		}
		return n, nil
	case "select":
		if v == nil || v == "" {
			if f.Required {
				return nil, i18n.Errorf("credential::field_required", f.Key)
			}
			return nil, nil
		}
		raw, _ := json.Marshal(v)
		for _, o := range f.Options {
			if string(o.Value) == string(raw) {
				return v, nil
			}
		}
		return nil, i18n.Errorf("credential::invalid_field", f.Key)
	default:
		s, _ := v.(string)
		s = strings.TrimSpace(s)
		if s == "" {
			if f.Required {
				return nil, i18n.Errorf("credential::field_required", f.Key)
			}
			return nil, nil
		}
		if max := maxLength(f); utf8.RuneCountInString(s) > max {
			return nil, i18n.Errorf("credential::too_long", f.Key, max)
		}
		if f.Widget == "email" {
			if addr, err := mail.ParseAddress(s); err != nil || addr.Address != s {
				return nil, i18n.Errorf("credential::invalid_field", f.Key)
			}
		}
		return s, nil
	}
}

// SealSecrets encrypts the secret fields, it returns an empty string if there are none.
func SealSecrets(secrets map[string]string) (string, error) {
	if len(secrets) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(secrets)
	if err != nil {
		return "", err
	}
	return sso.Seal(raw)
}

// OpenSecrets decrypts the secret fields.
func OpenSecrets(sealed string) (map[string]string, error) {
	out := map[string]string{}
	if sealed == "" {
		return out, nil
	}
	raw, err := sso.Open(sealed)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, errors.Wrap(err, "decode secrets")
	}
	return out, nil
}

// CredentialValues merges the plain data and the decrypted secrets.
func CredentialValues(data []byte, secrets map[string]string) map[string]interface{} {
	values := map[string]interface{}{}
	_ = json.Unmarshal(data, &values)
	for k, v := range secrets {
		values[k] = v
	}
	return values
}

func decodeValues[T any](values map[string]interface{}) (*T, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// smtpServer resolves the smtp credential referenced by an email step, which must belong to the current project.
func (x *execution) smtpServer(uid string) (*SMTPServer, error) {
	if uid == "" {
		return nil, &ActionError{Message: "no email credential is selected"}
	}
	c, err := db.NewCredentialsStore(x.db).GetByUID(x.ctx, uid)
	if err != nil {
		if errors.Is(err, db.ErrCredentialNotFound) {
			return nil, &ActionError{Message: "the email credential does not exist"}
		}
		return nil, err
	}
	if c.ProjectID != x.project.ID || c.Type != "smtp" {
		return nil, &ActionError{Message: "the email credential does not exist"}
	}
	secrets, err := OpenSecrets(c.Sealed)
	if err != nil {
		return nil, &ActionError{Message: "the email credential can not be decrypted"}
	}
	server, err := decodeValues[SMTPServer](CredentialValues(c.Data, secrets))
	if err != nil {
		return nil, errors.Wrap(err, "decode smtp credential")
	}
	return server, nil
}

// CredentialTestMessage holds the translated texts used to test a credential.
type CredentialTestMessage struct {
	Subject string
	Content string
}

// credentialTesters implement the test action of each credential type; input holds the values of test.fields in the DSL.
var credentialTesters = map[string]func(ctx context.Context, values map[string]interface{}, input map[string]string, msg CredentialTestMessage) error{
	"smtp": func(ctx context.Context, values map[string]interface{}, input map[string]string, msg CredentialTestMessage) error {
		server, err := decodeValues[SMTPServer](values)
		if err != nil {
			return err
		}
		return SendSMTP(ctx, *server, Mail{To: []string{input["to"]}, Subject: msg.Subject, Content: msg.Content}, networkAllowlist(ctx, db.Settings))
	},
}

// TestCredential runs the test action with the credential, failures are *ActionError that can be shown.
func TestCredential(ctx context.Context, def *CredentialTypeDef, values map[string]interface{}, input map[string]string, msg CredentialTestMessage) error {
	test := credentialTesters[def.Type]
	if def.Test == nil || test == nil {
		return &ActionError{Message: "the credential type can not be tested"}
	}
	clean := map[string]string{}
	for _, f := range def.Test.Fields {
		v := strings.TrimSpace(input[f.Key])
		if f.Required && v == "" {
			return i18n.Errorf("credential::field_required", f.Key)
		}
		if f.Widget == "email" && v != "" {
			addr, err := mail.ParseAddress(v)
			if err != nil {
				return i18n.Errorf("credential::invalid_field", f.Key)
			}
			v = addr.Address
		}
		clean[f.Key] = v
	}
	return test(ctx, values, clean, msg)
}
