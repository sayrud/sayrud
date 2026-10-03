package script

import (
	"strings"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

// aiComplete blocks like fetch and can be awaited by the script.
func (s *sandbox) aiComplete(call goja.FunctionCall) goja.Value {
	s.aiCalls++
	if s.aiCalls > maxRequests {
		panic(s.vm.NewTypeError("ai.complete: at most %d requests are allowed in a run", maxRequests))
	}
	if s.opts.AIComplete == nil {
		panic(s.hostError(ErrAIDisabled))
	}

	request, ok := call.Argument(0).Export().(map[string]interface{})
	if !ok || len(call.Arguments) != 1 {
		panic(s.vm.NewTypeError("ai.complete: expected one object with prompt and optional system"))
	}
	for key := range request {
		if key != "prompt" && key != "system" {
			panic(s.vm.NewTypeError("ai.complete: unsupported option %q", key))
		}
	}

	prompt, ok := request["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		panic(s.vm.NewTypeError("ai.complete: prompt must be a non-empty string"))
	}

	var system string
	if raw, present := request["system"]; present {
		var ok bool
		system, ok = raw.(string)
		if !ok {
			panic(s.vm.NewTypeError("ai.complete: system must be a string"))
		}
	}
	if len(prompt)+len(system) > maxRequestBody {
		panic(s.vm.NewTypeError("ai.complete: the request body is too large"))
	}

	response, err := s.opts.AIComplete(s.ctx, prompt, system)
	if s.ctx.Err() != nil {
		panic(s.hostError(s.ctxError()))
	}
	if err != nil {
		panic(s.hostError(err))
	}

	return s.vm.ToValue(response)
}

// Keep the original typed error in Go: GoError.value must only expose its safe message.
func (s *sandbox) hostError(err error) *goja.Object {
	object := s.vm.NewGoError(errors.New(err.Error()))
	if s.hostErrors == nil {
		s.hostErrors = make(map[*goja.Object]error)
	}
	s.hostErrors[object] = err

	return object
}
