package workflow

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// paramErrorVars are the variables a parameter error response may reference.
var paramErrorVars = []string{"error", "errors", "run"}

// wholePlaceholder matches a string that is a single placeholder, which keeps the variable's original type.
var wholePlaceholder = regexp.MustCompile(`^\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}$`)

// paramErrorScope returns the variables of a parameter error response; run.id is missing when runUID is empty.
func paramErrorScope(errs []ParamError, runUID string) Scope {
	list, _ := normalize(errs).([]interface{})
	run := map[string]interface{}{}
	if runUID != "" {
		run["id"] = runUID
	}
	scope := Scope{"errors": list, "run": run}
	if len(list) > 0 {
		scope["error"] = list[0]
	}
	return scope
}

// renderErrorResponse renders the placeholders in the string values of a custom response; it returns nil when the response is unset or invalid.
func renderErrorResponse(r *ErrorResponse, scope Scope) *Response {
	if r == nil || !r.validStatus() || !json.Valid([]byte(r.Body)) {
		return nil
	}
	resp := &Response{Status: r.Status, Headers: map[string]string{}}
	if !strings.Contains(r.Body, "{{") {
		resp.Body = json.RawMessage(r.Body)
		return resp
	}
	dec := json.NewDecoder(strings.NewReader(r.Body))
	dec.UseNumber()
	body, err := decodeOrdered(dec)
	if err != nil {
		return nil
	}
	resp.Body = renderJSON(body, scope)
	return resp
}

// decodeOrdered decodes the next JSON value, using *Object for objects to keep the property order.
func decodeOrdered(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		obj := NewObject()
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return nil, err
			}
			v, err := decodeOrdered(dec)
			if err != nil {
				return nil, err
			}
			obj.Set(key.(string), v)
		}
		_, err = dec.Token()
		return obj, err
	case json.Delim('['):
		list := []interface{}{}
		for dec.More() {
			v, err := decodeOrdered(dec)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err = dec.Token()
		return list, err
	}
	return tok, nil
}

// renderJSON replaces the placeholders in string values in place; a string that is a single placeholder becomes the variable's value, or null when it is missing.
func renderJSON(v interface{}, scope Scope) interface{} {
	switch t := v.(type) {
	case string:
		if m := wholePlaceholder.FindStringSubmatch(t); m != nil {
			out, _ := scope.Lookup(m[1])
			return out
		}
		return scope.Render(t)
	case *Object:
		for _, k := range t.keys {
			t.values[k] = renderJSON(t.values[k], scope)
		}
	case []interface{}:
		for i := range t {
			t[i] = renderJSON(t[i], scope)
		}
	}
	return v
}

// templateRoots returns the sorted root variables of the placeholders in the string values of a JSON text.
func templateRoots(body string) []string {
	var v interface{}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return nil
	}
	var roots []string
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case string:
			for _, m := range TemplatePattern.FindAllStringSubmatch(t, -1) {
				root, _, _ := strings.Cut(m[1], ".")
				if !slices.Contains(roots, root) {
					roots = append(roots, root)
				}
			}
		case map[string]interface{}:
			for _, item := range t {
				walk(item)
			}
		case []interface{}:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	slices.Sort(roots)
	return roots
}
