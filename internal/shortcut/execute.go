package shortcut

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/sso"
)

// AIClient completes the chat messages.
type AIClient interface {
	Complete(ctx context.Context, messages []openai.Message) (string, error)
}

// Env is where the shortcut is executed, it is passed to the scripts as the context.
type Env struct {
	ProjectUID string
	TableUID   string
	FieldUID   string
	RecordUID  string
}

// Executor generates the cell values.
type Executor struct {
	// AI returns the client of the configured model, or nil if not configured.
	AI func(ctx context.Context) (AIClient, error)
}

// NewExecutor returns the executor using the model configured in the admin console.
func NewExecutor() *Executor {
	return &Executor{AI: LoadAIClient}
}

// Execute generates the cell value of the field by its shortcut from the record data, which is keyed by field UID.
// It returns nil without executing if all the referenced fields are empty, and *Error if the execution fails.
func (e *Executor) Execute(ctx context.Context, def *Definition, fields []*db.SLField, field *db.SLField, data map[string]interface{}, env Env) (interface{}, error) {
	if field.Shortcut == nil {
		return nil, newError("shortcut::invalid_config")
	}

	params, texts, empty, err := resolveInputs(def, fields, field, data)
	if err != nil {
		return nil, err
	}
	if empty {
		return nil, nil
	}

	switch def.Kind {
	case KindAI:
		return e.executeAI(ctx, def, field, texts)
	case KindScript:
		secrets, err := DecodeSecrets(def.custom.Secrets)
		if err != nil {
			return nil, newError("shortcut::secrets_unavailable")
		}
		if !def.custom.Enabled {
			return nil, newError("shortcut::disabled")
		}
		result, err := RunCustom(ctx, def.custom, secrets, params, env)
		if err != nil {
			return nil, err
		}
		return CellValue(field, result.Value)
	}
	return nil, newError("shortcut::not_found")
}

// resolveInputs returns the parameters and their texts from the record data, empty reports whether the shortcut reads
// any field and all of them are empty.
func resolveInputs(def *Definition, fields []*db.SLField, field *db.SLField, data map[string]interface{}) (params map[string]interface{}, texts map[string]string, empty bool, err error) {
	byUID := lo.KeyBy(fields, func(f *db.SLField) string { return f.UID })
	params = make(map[string]interface{}, len(def.FormItems))
	texts = make(map[string]string, len(def.FormItems))
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

	for _, item := range def.FormItems {
		raw, _ := field.Shortcut.Inputs[item.Key].(string)
		switch item.Component {
		case db.ShortcutFormFieldSelect:
			if raw == "" {
				if item.Required {
					return nil, nil, false, newError("shortcut::input_required", item.Label)
				}
				params[item.Key] = nil
				continue
			}
			value, ok := read(raw)
			if !ok {
				return nil, nil, false, newError("shortcut::invalid_input_field", item.Label)
			}
			params[item.Key] = value
			texts[item.Key] = InputText(value)
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
				return nil, nil, false, newError("shortcut::invalid_input_field", item.Label)
			}
			params[item.Key] = text
			texts[item.Key] = text
		default:
			params[item.Key] = raw
			texts[item.Key] = raw
		}
	}
	return params, texts, readsFields && !anyFilled, nil
}

func (e *Executor) executeAI(ctx context.Context, def *Definition, field *db.SLField, texts map[string]string) (interface{}, error) {
	client, err := e.AI(ctx)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, newError("shortcut::ai_unavailable")
	}

	messages, err := def.builtin.prompt(promptInput{field: field, texts: texts})
	if err != nil {
		return nil, err
	}
	completion, err := client.Complete(ctx, messages)
	if err != nil {
		return nil, modelError(err)
	}

	result, err := parseCompletion(field, completion)
	if err != nil {
		return nil, err
	}
	return CellValue(field, result)
}

func modelError(err error) *Error {
	var statusErr *openai.StatusError
	if errors.As(err, &statusErr) {
		if statusErr.Temporary() {
			return newError("shortcut::model_unavailable").withDetail("%s", statusErr.Error()).transient()
		}
		return newError("shortcut::model_error").withDetail("%s", statusErr.Error())
	}

	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
		return newError("shortcut::model_unavailable").withDetail("%s", errors.Cause(err).Error()).transient()
	}
	return newError("shortcut::model_error").withDetail("%s", errors.Cause(err).Error())
}

// RunCustom runs the script of the custom shortcut with the parameters, the logs are returned along with the error if available.
func RunCustom(ctx context.Context, custom *db.CustomFieldShortcut, secrets map[string]string, params map[string]interface{}, env Env) (*script.Result, error) {
	credentials := make([]script.Credential, 0, len(custom.Credentials.Data()))
	for _, c := range custom.Credentials.Data() {
		credentials = append(credentials, script.Credential{Key: c.Key, Type: c.Type, Name: c.Name, Value: secrets[c.Key]})
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
		Credentials: credentials,
		Timeout:     time.Duration(custom.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		return result, scriptError(err)
	}
	return result, nil
}

func scriptError(err error) *Error {
	var scriptErr *script.Error
	switch {
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
