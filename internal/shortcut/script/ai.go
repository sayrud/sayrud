package script

import (
	"strings"

	"github.com/Calcium-Ion/moejs"
)

// aiComplete blocks like fetch and can be awaited by the script.
func (s *sandbox) aiComplete(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	s.aiCalls++
	if s.aiCalls > maxRequests {
		return moejs.Undefined(), r.TypeError("ai.complete: at most %d requests are allowed in a run", maxRequests)
	}

	if s.opts.AIComplete == nil {
		return moejs.Undefined(), s.hostError(ErrAIDisabled)
	}

	value := moejs.Arg(args, 0)
	if len(args) != 1 || !value.IsObject() || value.AsObject().ClassName() != "Object" || value.AsObject().IsCallable() {
		return moejs.Undefined(), r.TypeError("ai.complete: expected one object with prompt and optional system")
	}

	// Read each option once and preserve its JavaScript type; ToGo unwraps boxed strings.
	keys, err := r.EnumerableOwnKeys(value.AsObject())
	if err != nil {
		return moejs.Undefined(), err
	}
	request := make(map[string]moejs.Value, len(keys))
	for _, key := range keys {
		name := key.GoString()
		if name != "prompt" && name != "system" {
			return moejs.Undefined(), r.TypeError("ai.complete: unsupported option %q", name)
		}
		option, err := s.vm.Get(value, name)
		if err != nil {
			return moejs.Undefined(), err
		}
		request[name] = option
	}

	promptValue, present := request["prompt"]
	if !present || !promptValue.IsString() || strings.TrimSpace(promptValue.String()) == "" {
		return moejs.Undefined(), r.TypeError("ai.complete: prompt must be a non-empty string")
	}
	prompt := promptValue.String()

	var system string
	if value, present := request["system"]; present {
		if !value.IsString() {
			return moejs.Undefined(), r.TypeError("ai.complete: system must be a string")
		}
		system = value.String()
	}

	if len(prompt)+len(system) > maxRequestBody {
		return moejs.Undefined(), r.TypeError("ai.complete: the request body is too large")
	}

	response, err := s.opts.AIComplete(s.ctx, prompt, system)
	if s.ctx.Err() != nil {
		return moejs.Undefined(), s.hostError(s.vm.ContextError())
	}
	if err != nil {
		return moejs.Undefined(), s.hostError(err)
	}

	return moejs.String(response), nil
}

// Keep GoError.value available to old scripts without exposing the original Go error's fields.
func (s *sandbox) hostError(err error) error {
	value, ok := s.vm.ThrownValue(err)
	if !ok {
		return err
	}
	safe, convertErr := s.vm.FromGo(map[string]any{})
	if convertErr != nil {
		return convertErr
	}

	r := s.vm.Realm()
	if setErr := value.AsObject().SetProp(r, r.KeyFromGoString("value"), safe); setErr != nil {
		return setErr
	}
	if setErr := value.AsObject().SetProp(r, r.KeyFromGoString("name"), moejs.String("GoError")); setErr != nil {
		return setErr
	}

	return &moejs.Exception{Value: value}
}
