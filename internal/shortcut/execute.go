package shortcut

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/sso"
)

// Env is where the shortcut is executed, it is passed to the scripts as the context.
type Env struct {
	ProjectUID string
	TableUID   string
	FieldUID   string
	RecordUID  string
}

// Execute generates the cell value of the field by its shortcut from the record data, which is keyed by field UID.
// It returns nil without executing if all the referenced fields are empty, and *Error if the execution fails.
func Execute(ctx context.Context, custom *db.CustomFieldShortcut, fields []*db.SLField, field *db.SLField, data map[string]interface{}, env Env) (interface{}, error) {
	if field.Shortcut == nil {
		return nil, newError("shortcut::invalid_config")
	}

	md, _ := field.Metadata.Data().(map[string]interface{})
	if md["optionsReference"] != nil {
		var err error
		field, err = db.SLFields.FilterOptions(ctx, field, fields, data)
		if err != nil {
			return nil, newError("field::invalid_options_reference")
		}
		if len(optionNames(field)) == 0 {
			return nil, nil
		}
	}

	params, empty, err := resolveInputs(custom, fields, field, data)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, nil
	}

	secrets, err := DecodeSecrets(custom.Secrets)
	if err != nil {
		return nil, newError("shortcut::secrets_unavailable")
	}
	if !custom.Enabled {
		return nil, newError("shortcut::disabled")
	}
	result, err := RunCustom(ctx, custom, secrets, params, env)
	if err != nil {
		return nil, err
	}
	return CellValue(field, result.Value)
}

// resolveInputs returns the parameters from the record data, empty reports whether the shortcut reads
// any field and all of them are empty.
func resolveInputs(custom *db.CustomFieldShortcut, fields []*db.SLField, field *db.SLField, data map[string]interface{}) (params map[string]interface{}, empty bool, err error) {
	byUID := lo.KeyBy(fields, func(f *db.SLField) string { return f.UID })
	params = make(map[string]interface{}, len(custom.FormItems.Data()))
	readsFields, anyFilled := false, false

	read := func(uid string) (interface{}, bool) {
		ref, ok := byUID[uid]
		if !ok || ref.UID == field.UID || ref.Type == db.FormulaFieldType {
			return nil, false
		}
		readsFields = true
		value := InputValue(ref, data[uid])
		if !isEmptyInput(value) {
			anyFilled = true
		}
		return value, true
	}

	for _, item := range custom.FormItems.Data() {
		raw, _ := field.Shortcut.Inputs[item.Key].(string)
		switch item.Component {
		case db.ShortcutFormFieldSelect:
			if raw == "" {
				if item.Required {
					return nil, false, newError("shortcut::input_required", item.Label)
				}
				params[item.Key] = nil
				continue
			}
			value, ok := read(raw)
			if !ok {
				return nil, false, newError("shortcut::invalid_input_field", item.Label)
			}
			params[item.Key] = value
		case db.ShortcutFormPrompt:
			var invalid bool
			text := fieldRefPattern.ReplaceAllStringFunc(raw, func(m string) string {
				value, ok := read(m[1 : len(m)-1])
				if !ok {
					invalid = true
				}
				return InputText(value)
			})
			if invalid {
				return nil, false, newError("shortcut::invalid_input_field", item.Label)
			}
			params[item.Key] = text
		case db.ShortcutFormFieldOptions:
			params[item.Key] = optionNames(field)
		default:
			params[item.Key] = raw
		}
	}
	return params, readsFields && !anyFilled, nil
}

// RunCustom runs the script of the custom shortcut with the parameters, the logs are returned along with the error if available.
func RunCustom(ctx context.Context, custom *db.CustomFieldShortcut, secrets map[string]string, params map[string]interface{}, env Env) (*script.Result, error) {
	var networks []netip.Prefix
	if len(custom.Domains.Data()) > 0 {
		settings, err := db.Settings.GetSystem(ctx)
		if err != nil {
			return nil, newError("shortcut::internal_error").transient()
		}
		networks, err = db.ParseNetworkAllowlist(settings.NetworkAllowlist)
		if err != nil {
			return nil, newError("shortcut::internal_error")
		}
	}

	credentials := make([]script.Credential, 0, len(custom.Credentials.Data()))
	for _, c := range custom.Credentials.Data() {
		credentials = append(credentials, script.Credential{Key: c.Key, Type: c.Type, Name: c.Name, Value: secrets[c.Key]})
	}

	var complete func(context.Context, string, string) (string, error)
	if custom.AIEnabled {
		complete = aiCompleter()
	}

	result, err := script.Run(ctx, script.Options{
		Code:   custom.Code,
		Params: params,
		Context: map[string]interface{}{
			"projectUID": env.ProjectUID,
			"tableUID":   env.TableUID,
			"fieldUID":   env.FieldUID,
			"recordUID":  env.RecordUID,
		},
		Domains:     custom.Domains.Data(),
		Networks:    networks,
		Credentials: credentials,
		Timeout:     time.Duration(custom.TimeoutSeconds) * time.Second,
		AIComplete:  complete,
	})
	if err != nil {
		return result, scriptError(err)
	}

	return result, nil
}

func scriptError(err error) *Error {
	var shortcutErr *Error
	var scriptErr *script.Error

	switch {
	case errors.As(err, &shortcutErr):
		return shortcutErr
	case errors.Is(err, script.ErrAIDisabled):
		return newError("shortcut::ai_disabled")
	case errors.As(err, &scriptErr):
		return newError("shortcut::script_error").withDetail("%s", scriptErr.Message)
	case errors.Is(err, script.ErrTimeout):
		return newError("shortcut::timeout")
	case errors.Is(err, script.ErrNoExecute):
		return newError("shortcut::no_execute")
	case errors.Is(err, context.Canceled):
		return newError("shortcut::canceled").transient()
	default:
		return newError("shortcut::script_error").withDetail("%s", err.Error())
	}
}

// EncodeSecrets encrypts the credential values keyed by credential key, it returns empty if there is none.
func EncodeSecrets(secrets map[string]string) (string, error) {
	secrets = lo.PickBy(secrets, func(_ string, v string) bool { return v != "" })
	if len(secrets) == 0 {
		return "", nil
	}
	raw, err := json.Marshal(secrets)
	if err != nil {
		return "", errors.Wrap(err, "marshal")
	}
	return sso.Seal(raw)
}

// DecodeSecrets decrypts the result of EncodeSecrets.
func DecodeSecrets(sealed string) (map[string]string, error) {
	secrets := map[string]string{}
	if sealed == "" {
		return secrets, nil
	}
	raw, err := sso.Open(sealed)
	if err != nil {
		return nil, err
	}
	return secrets, errors.Wrap(json.Unmarshal(raw, &secrets), "unmarshal")
}
