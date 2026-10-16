package workflow

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// Scope holds the variables a flow can reference; all values are JSON-compatible types.
//
// Top-level variables: steps (step outputs) and run (execution info); API triggers add input (validated parameters) and request (request snapshot),
// the async succeeded event adds response (the sync response); other triggers add trigger (the triggering record or time); inside map nodes, item (the current element) is added.
type Scope map[string]interface{}

// Child returns a child scope with the item variable.
func (s Scope) Child(item interface{}) Scope {
	child := make(Scope, len(s)+1)
	for k, v := range s {
		child[k] = v
	}
	child["item"] = item
	return child
}

// SetStep records a step's output.
func (s Scope) SetStep(id string, output interface{}) {
	steps, _ := s["steps"].(map[string]interface{})
	if steps == nil {
		steps = map[string]interface{}{}
		s["steps"] = steps
	}
	steps[id] = output
}

// Lookup returns the value at a dot-separated path, using numeric indexes for arrays; ok is false if the path does not exist.
func (s Scope) Lookup(path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	current, ok := s[parts[0]]
	if !ok {
		return nil, false
	}
	for _, part := range parts[1:] {
		switch v := current.(type) {
		case map[string]interface{}:
			current, ok = v[part]
		case *Object:
			current, ok = v.Get(part)
		case []interface{}:
			if part == "length" {
				current, ok = float64(len(v)), true
				break
			}
			i, err := strconv.Atoi(part)
			ok = err == nil && i >= 0 && i < len(v)
			if ok {
				current = v[i]
			}
		default:
			ok = false
		}
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// Resolve evaluates a value; it returns nil for a nil or unknown value.
func (s Scope) Resolve(v *Value) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	switch v.Kind {
	case ValueConst:
		if len(v.Const) == 0 {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal(v.Const, &out); err != nil {
			return nil, errors.Wrap(err, "decode constant")
		}
		return out, nil
	case ValueVar:
		out, _ := s.Lookup(v.Var)
		return out, nil
	case ValueTemplate:
		return s.Render(v.Template), nil
	case ValueFormula:
		return s.evalFormula(v.Formula)
	}
	return nil, nil
}

// TemplatePattern matches {{variable}} placeholders in a template.
var TemplatePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

// Render replaces the variables in a template with text; missing variables become empty strings.
func (s Scope) Render(template string) string {
	return TemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		path := TemplatePattern.FindStringSubmatch(match)[1]
		v, _ := s.Lookup(path)
		return Text(v)
	})
}

// Text converts a value to text: strings are returned as is, nil becomes an empty string, and everything else becomes JSON.
func Text(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

// Object is a JSON object that preserves property order.
type Object struct {
	keys   []string
	values map[string]interface{}
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{values: map[string]interface{}{}}
}

// Set sets a property; an existing property keeps its original position.
func (o *Object) Set(key string, value interface{}) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// Get returns a property value.
func (o *Object) Get(key string) (interface{}, bool) {
	v, ok := o.values[key]
	return v, ok
}

func (o *Object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		value, err := json.Marshal(o.values[k])
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// maxMapItems limits the number of elements a map node produces, to avoid building oversized responses.
const maxMapItems = 1000

// BuildBody builds a JSON value from a structured definition.
func (s Scope) BuildBody(b *Body) (interface{}, error) {
	if b == nil {
		return nil, nil
	}
	switch b.Type {
	case BodyObject:
		obj := NewObject()
		for i := range b.Fields {
			f := &b.Fields[i]
			if f.Key == "" {
				continue
			}
			v, err := s.BuildBody(&f.Value)
			if err != nil {
				return nil, err
			}
			obj.Set(f.Key, v)
		}
		return obj, nil
	case BodyValue:
		return s.Resolve(b.Value)
	case BodyMap:
		source, err := s.Resolve(b.Source)
		if err != nil {
			return nil, err
		}
		list, _ := source.([]interface{})
		out := make([]interface{}, 0, len(list))
		for i, item := range list {
			if i >= maxMapItems {
				break
			}
			v, err := s.Child(item).BuildBody(b.Item)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return nil, nil
}

// normalize converts a value to plain JSON types so it can later be read by path.
func normalize(v interface{}) interface{} {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out interface{}
	_ = json.Unmarshal(raw, &out)
	return out
}
