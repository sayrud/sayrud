package workflow

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/db"
)

// Templates for new API workflows.
const (
	TemplateBlank  = "blank"
	TemplateCreate = "create"
	TemplateQuery  = "query"
)

// NewID returns a random identifier for a parameter or flow.
func NewID(prefix string) string {
	return prefix + strings.ToLower(randstr.String(8))
}

func constValue(v interface{}) Value {
	raw, _ := json.Marshal(v)
	return Value{Kind: ValueConst, Const: raw}
}

func varValue(path string) *Value {
	return &Value{Kind: ValueVar, Var: path}
}

// slug converts a name to a lowercase ASCII identifier, or returns an empty string if it can not be converted.
func slug(name string, sep rune) string {
	var b strings.Builder
	lastSep := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastSep = false
		case !lastSep:
			b.WriteRune(sep)
			lastSep = true
		}
	}
	return strings.Trim(b.String(), string(sep))
}

// BlankSpec returns the draft of a blank API workflow: a POST request that simply returns success.
func BlankSpec(path string) *Spec {
	return &Spec{
		Trigger: Trigger{Type: TriggerAPI, API: &APITrigger{
			Method: "POST", Path: path, Auth: Auth{Type: AuthNone}, Params: []Param{}, RateLimit: RateLimit{Limit: 60, Window: 60},
		}},
		Steps: []Step{{
			ID: "respond", Type: StepResponse, Status: 200,
			Body: &Body{Type: BodyObject, Fields: []BodyField{{Key: "ok", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueConst, Const: json.RawMessage("true")}}}}},
		}},
		Async: []AsyncFlow{},
	}
}

// TriggerSpec returns a blank draft triggered by records, dates or a schedule; timezone is the schedule trigger's time zone, and now determines the first fire time.
func TriggerSpec(triggerType string, table *db.SLTable, fields []*db.SLField, timezone string, now time.Time) *Spec {
	spec := &Spec{Trigger: Trigger{Type: triggerType}, Steps: []Step{}, Async: []AsyncFlow{}}
	if triggerType == TriggerSchedule {
		loc, err := time.LoadLocation(timezone)
		if err != nil || timezone == "" {
			loc, timezone = time.UTC, "UTC"
		}
		next := now.In(loc).Truncate(time.Hour).Add(time.Hour)
		spec.Trigger.Schedule = &ScheduleTrigger{Start: next.Format(ScheduleLayout), Timezone: timezone, Repeat: RepeatDaily}
		return spec
	}
	r := &RecordTrigger{Match: "all"}
	if table != nil {
		r.TableUID = table.UID
	}
	if triggerType == TriggerDateReached {
		for _, f := range fields {
			if f.Type == db.DateTimeFieldType {
				r.DateFieldUID = f.UID
				break
			}
		}
	}
	spec.Trigger.Record = r
	return spec
}

// paramForField returns a parameter matching the field's type.
func paramForField(field *db.SLField, key, source string) (Param, bool) {
	p := Param{ID: NewID("p"), Key: key, Source: source, Description: field.Label}
	switch field.Type {
	case db.TextFieldType:
		p.Type = TypeString
	case db.NumberFieldType:
		p.Type = TypeNumber
	case db.CheckboxFieldType:
		p.Type = TypeBoolean
	case db.DateTimeFieldType:
		p.Type = TypeDateTime
	case db.SingleSelectFieldType:
		p.Type = TypeEnum
		for _, o := range fieldOptions(field) {
			p.Options = append(p.Options, o.Name)
		}
		if len(p.Options) == 0 {
			p.Type = TypeString
		}
	case db.MultiSelectFieldType:
		p.Type, p.ItemType = TypeArray, TypeString
		for _, o := range fieldOptions(field) {
			p.Options = append(p.Options, o.Name)
		}
	default:
		return Param{}, false
	}
	return p, true
}

// TableSpec generates a ready-to-debug draft for a table: create writes a record and returns it, and query queries by conditions and returns a list.
func TableSpec(template string, table *db.SLTable, fields []*db.SLField) *Spec {
	base := slug(table.Name, '-')
	if base == "" {
		base = "records"
	}
	spec := BlankSpec("/" + base)
	spec.Steps = []Step{}

	used := map[string]bool{}
	keyFor := func(field *db.SLField, index int) string {
		key := slug(field.Label, '_')
		if key == "" || !paramKeyPattern.MatchString(key) {
			key = "field" + strconv.Itoa(index+1)
		}
		for base, n := key, 2; used[key]; n++ {
			key = base + strconv.Itoa(n)
		}
		used[key] = true
		return key
	}

	switch template {
	case TemplateCreate:
		create := Step{ID: "create", Type: StepCreate, TableUID: table.UID}
		for i, field := range fields {
			if !WritableField(field) {
				continue
			}
			p, ok := paramForField(field, keyFor(field, i), SourceBody)
			if !ok {
				continue
			}
			p.Required = i == 0
			spec.Trigger.API.Params = append(spec.Trigger.API.Params, p)
			create.Fields = append(create.Fields, FieldValue{FieldUID: field.UID, Value: *varValue("input." + p.Key)})
		}
		spec.Steps = append(spec.Steps, create, Step{
			ID: "respond", Type: StepResponse, Status: 201,
			Body: &Body{Type: BodyObject, Fields: []BodyField{
				{Key: "id", Value: Body{Type: BodyValue, Value: varValue("steps.create.record.uid")}},
				{Key: "createdAt", Value: Body{Type: BodyValue, Value: varValue("steps.create.record.createdAt")}},
			}},
		})

	case TemplateQuery:
		spec.Trigger.API.Method = "GET"
		query := Step{ID: "query", Type: StepQuery, TableUID: table.UID, Limit: DefaultQueryLimit}
		item := &Body{Type: BodyObject, Fields: []BodyField{{Key: "id", Value: Body{Type: BodyValue, Value: varValue("item.uid")}}}}
		for i, field := range fields {
			if field.Type == db.FormulaFieldType || field.Type == db.AttachmentFieldType || field.Type.IsUnknown() {
				continue
			}
			key := keyFor(field, i)
			item.Fields = append(item.Fields, BodyField{Key: key, Value: Body{Type: BodyValue, Value: varValue("item.fields." + field.UID)}})
			// The first text field serves as an optional filter condition.
			if field.Type == db.TextFieldType && len(query.Filters) == 0 {
				p := Param{ID: NewID("p"), Key: key, Source: SourceQuery, Type: TypeString}
				spec.Trigger.API.Params = append(spec.Trigger.API.Params, p)
				query.Filters = append(query.Filters, Filter{FieldUID: field.UID, Op: "like", Value: varValue("input." + key)})
			}
		}
		spec.Steps = append(spec.Steps, query, Step{
			ID: "respond", Type: StepResponse, Status: 200,
			Body: &Body{Type: BodyObject, Fields: []BodyField{
				{Key: "total", Value: Body{Type: BodyValue, Value: varValue("steps.query.total")}},
				{Key: "records", Value: Body{Type: BodyMap, Source: varValue("steps.query.records"), Item: item}},
			}},
		})
	}
	return spec
}
