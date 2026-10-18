// Package workflow defines, checks and executes workflows: step-by-step logic triggered by API requests, record changes, dates in records, or schedules.
package workflow

import (
	"encoding/json"
)

// Spec is the complete configuration of a workflow; the draft and every published revision each store a Spec.
type Spec struct {
	Trigger Trigger `json:"trigger"`
	// Steps is the main flow: executed synchronously to build the response for API triggers, and in the background for other triggers.
	Steps []Step `json:"steps"`
	// Async holds the flows an API trigger runs in the background after request events; empty for other triggers.
	Async []AsyncFlow `json:"async"`
	// Timezone is the IANA time zone in which formulas handle dates, e.g. Asia/Shanghai; UTC is used when it is empty or invalid.
	Timezone string `json:"timezone,omitempty"`
}

// FormulaTimezone returns the time zone of the formulas: the schedule trigger's time zone takes precedence over Timezone.
func (s *Spec) FormulaTimezone() string {
	if s.Trigger.Type == TriggerSchedule && s.Trigger.Schedule != nil && s.Trigger.Schedule.Timezone != "" {
		return s.Trigger.Schedule.Timezone
	}
	return s.Timezone
} // @name WorkflowSpec

// Trigger types.
const (
	TriggerAPI           = "api"
	TriggerRecordCreated = "record_created"
	TriggerRecordUpdated = "record_updated"
	TriggerRecordMatched = "record_matched"
	TriggerRecordDeleted = "record_deleted"
	TriggerDateReached   = "date_reached"
	TriggerSchedule      = "schedule"
)

// TriggerTypes lists the supported trigger types.
var TriggerTypes = []string{TriggerAPI, TriggerRecordCreated, TriggerRecordUpdated, TriggerRecordMatched, TriggerRecordDeleted, TriggerDateReached, TriggerSchedule}

// IsRecordTrigger reports whether the trigger type fires on record creation, update or deletion.
func IsRecordTrigger(t string) bool {
	return t == TriggerRecordCreated || t == TriggerRecordUpdated || t == TriggerRecordMatched || t == TriggerRecordDeleted
}

// Trigger is the workflow trigger and keeps only the configuration matching Type; an empty Type means a new workflow has not chosen a trigger yet.
type Trigger struct {
	Type string `json:"type" enums:",api,record_created,record_updated,record_matched,record_deleted,date_reached,schedule"`
	// Name is the display name of the trigger node; when empty, the trigger type's name is shown.
	Name string `json:"name,omitempty"`
	// API is the request endpoint, authentication and parameters of an api trigger.
	API *APITrigger `json:"api,omitempty"`
	// Record is the table watched by record triggers and date_reached.
	Record *RecordTrigger `json:"record,omitempty"`
	// Schedule is the execution time of a schedule trigger.
	Schedule *ScheduleTrigger `json:"schedule,omitempty"`
} // @name WorkflowTrigger

// APITrigger is the endpoint that fires when an API request is received.
type APITrigger struct {
	// Method is GET, POST, PUT, PATCH or DELETE.
	Method string `json:"method"`
	// Path is a static path relative to the project, e.g. /leads.
	Path      string    `json:"path"`
	Auth      Auth      `json:"auth"`
	RateLimit RateLimit `json:"rateLimit"`
	Params    []Param   `json:"params"`
} // @name WorkflowAPITrigger

// RecordTrigger is the table and conditions of a record trigger.
type RecordTrigger struct {
	TableUID string `json:"tableUID"`
	// Fields are the field UIDs watched by record_updated; empty means any field.
	Fields []string `json:"fields,omitempty"`
	// Filters are the conditions a record must satisfy and may only use constant values; record_matched requires at least one condition, and Match is all or any.
	Filters []Filter `json:"filters,omitempty"`
	Match   string   `json:"match,omitempty" enums:"all,any"`
	// DateFieldUID is the date field for date_reached, and Offset is the offset in minutes from the field's time; negative means earlier.
	DateFieldUID string `json:"dateFieldUID,omitempty"`
	Offset       int    `json:"offset,omitempty"`
} // @name WorkflowRecordTrigger

// Repeat modes of schedule triggers.
const (
	RepeatNone     = "none"
	RepeatMinutes  = "minutes"
	RepeatHours    = "hours"
	RepeatDaily    = "daily"
	RepeatWorkdays = "workdays"
	RepeatWeekly   = "weekly"
	RepeatMonthly  = "monthly"
)

// ScheduleTrigger runs from Start and repeats per Repeat, with times interpreted in Timezone.
type ScheduleTrigger struct {
	// Start is the local time of the first run, formatted as 2006-01-02T15:04.
	Start    string `json:"start"`
	Timezone string `json:"timezone"`
	Repeat   string `json:"repeat" enums:"none,minutes,hours,daily,workdays,weekly,monthly"`
	// Interval is the interval for minutes and hours; a minute interval is at least 5.
	Interval int `json:"interval,omitempty"`
	// Weekdays are the days of the week for weekly runs; 0 is Sunday.
	Weekdays []int `json:"weekdays,omitempty"`
} // @name WorkflowScheduleTrigger

// Authentication types.
const (
	AuthNone   = "none"
	AuthAPIKey = "api_key"
)

// Auth is how callers authenticate.
type Auth struct {
	// Type is none or api_key; the key is passed via X-API-Key or Authorization: Bearer.
	Type string `json:"type" enums:"none,api_key"`
} // @name WorkflowAuth

// RateLimit limits requests per client IP within a fixed window.
type RateLimit struct {
	Enabled bool `json:"enabled"`
	// Limit is the maximum number of requests allowed per window.
	Limit int `json:"limit"`
	// Window is the window length in seconds.
	Window int `json:"window"`
} // @name WorkflowRateLimit

// Parameter sources.
const (
	SourceQuery  = "query"
	SourceHeader = "header"
	SourceBody   = "body"
)

// Parameter types.
const (
	TypeString   = "string"
	TypeNumber   = "number"
	TypeInteger  = "integer"
	TypeBoolean  = "boolean"
	TypeDate     = "date"
	TypeDateTime = "datetime"
	TypeEnum     = "enum"
	TypeArray    = "array"
)

// Formats of string parameters.
const (
	FormatEmail = "email"
	FormatURL   = "url"
	FormatPhone = "phone"
)

// Param is a declared request parameter whose validated value is referenced as input.<key>.
type Param struct {
	// ID is the stable identifier of the parameter; renaming the parameter does not change it.
	ID string `json:"id"`
	// Key is the parameter's name in its source, e.g. a JSON property name or a query parameter name.
	Key    string `json:"key"`
	Source string `json:"source" enums:"query,header,body"`
	Type   string `json:"type" enums:"string,number,integer,boolean,date,datetime,enum,array"`
	// ItemType is the array element type, string or number.
	ItemType    string `json:"itemType,omitempty" enums:"string,number"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	// Default is filled in only when the parameter is missing and must match the type.
	Default json.RawMessage `json:"default,omitempty" swaggertype:"object"`

	MinLength *int     `json:"minLength,omitempty"`
	MaxLength *int     `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MaxItems  *int     `json:"maxItems,omitempty"`
	// Pattern is the regular expression a string must match.
	Pattern string `json:"pattern,omitempty"`
	Format  string `json:"format,omitempty" enums:"email,url,phone"`
	// Options are the allowed enum values; when non-empty, they also constrain the elements of string arrays.
	Options []string `json:"options,omitempty"`
} // @name WorkflowParam

// Step types.
const (
	StepQuery     = "query"
	StepCreate    = "create"
	StepUpdate    = "update"
	StepCondition = "condition"
	StepResponse  = "response"
	StepEmail     = "email"
	StepHTTP      = "http"
)

// Step is a node in a flow, its output is referenced as steps.<id>.
type Step struct {
	ID string `json:"id"`
	// Type is a built-in module (query, create, update, condition, response, email, http) or an installed plugin (plugin:<id>).
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	// Config is the module configuration defined by the DSL of the type (see ModuleDef), decoded as the matching *XxxConfig.
	Config interface{} `json:"config" swaggertype:"object"`
	// Then and Else are the branches of a branching module.
	Then []Step `json:"then,omitempty"`
	Else []Step `json:"else,omitempty"`
} // @name WorkflowStep

// Value kinds.
const (
	ValueConst    = "const"
	ValueVar      = "var"
	ValueTemplate = "template"
	// ValueFormula is a formula that references variables as {path}.
	ValueFormula = "formula"
)

// Value is a constant, a variable reference (e.g. input.email), or a text template with {{variable}} placeholders.
type Value struct {
	Kind     string          `json:"kind" enums:"const,var,template,formula"`
	Const    json.RawMessage `json:"const,omitempty" swaggertype:"object"`
	Var      string          `json:"var,omitempty"`
	Template string          `json:"template,omitempty"`
	Formula  string          `json:"formula,omitempty"`
} // @name WorkflowValue

// Filter compares a record field; the value is passed as a query parameter.
type Filter struct {
	FieldUID string `json:"fieldUID"`
	Op       string `json:"op" enums:"eq,neq,gt,gte,lt,lte,like,in,nin,empty,not_empty"`
	Value    *Value `json:"value,omitempty"`
} // @name WorkflowFilter

// Sort orders records by a field.
type Sort struct {
	FieldUID string `json:"fieldUID"`
	Order    string `json:"order" enums:"asc,desc"`
} // @name WorkflowSort

// FieldValue maps a value to a field.
type FieldValue struct {
	FieldUID string `json:"fieldUID"`
	Value    Value  `json:"value"`
} // @name WorkflowFieldValue

// Condition compares two values.
type Condition struct {
	Left  Value  `json:"left"`
	Op    string `json:"op" enums:"eq,neq,gt,gte,lt,lte,contains,not_contains,empty,not_empty"`
	Right *Value `json:"right,omitempty"`
} // @name WorkflowCondition

// Header is a response or request header.
type Header struct {
	Name  string `json:"name"`
	Value Value  `json:"value"`
} // @name WorkflowHeader

// Body node types.
const (
	BodyObject = "object"
	BodyValue  = "value"
	BodyMap    = "map"
)

// Body is a node of a structured JSON body; numbers, booleans and arrays keep their original types.
type Body struct {
	Type string `json:"type" enums:"object,value,map"`
	// Fields are the properties of an object.
	Fields []BodyField `json:"fields,omitempty"`
	// Value is the value of a value node.
	Value *Value `json:"value,omitempty"`
	// Source is the array a map node iterates over, and Item builds each element using the variable item.
	Source *Value `json:"source,omitempty"`
	Item   *Body  `json:"item,omitempty"`
} // @name WorkflowBody

// BodyField is a property of an object body.
type BodyField struct {
	Key   string `json:"key"`
	Value Body   `json:"value"`
} // @name WorkflowBodyField

// Events that start the async flows of an API trigger.
const (
	// EventRequestReceived fires after routing, authentication, rate limiting and request parsing pass, but before parameter validation; it is saved in a separate transaction, so it runs even if later processing fails.
	EventRequestReceived = "request_received"
	// EventSucceeded fires when the sync flow succeeds and is saved in the same transaction as the business data.
	EventSucceeded = "succeeded"
)

// MainFlow is the flow ID of jobs that run the main flow in the background.
const MainFlow = "main"

// AsyncFlow runs in the background at least once after its event.
type AsyncFlow struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Event string `json:"event" enums:"request_received,succeeded"`
	Steps []Step `json:"steps"`
} // @name WorkflowAsyncFlow

// ParseSpec decodes a Spec from JSON, fills missing collections with empty ones, and keeps only the configuration matching the trigger type.
func ParseSpec(raw []byte) (*Spec, error) {
	var spec Spec
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, err
		}
	}
	spec.Normalize()
	return &spec, nil
}

// Normalize fills in empty collections and clears configuration unrelated to the trigger type: non-API triggers have no request endpoint or async flows.
func (s *Spec) Normalize() {
	if s.Steps == nil {
		s.Steps = []Step{}
	}
	if s.Async == nil {
		s.Async = []AsyncFlow{}
	}
	t := &s.Trigger
	switch t.Type {
	case "":
		t.API, t.Record, t.Schedule = nil, nil, nil
		s.Async = []AsyncFlow{}
	case TriggerAPI:
		if t.API == nil {
			t.API = &APITrigger{Method: "POST", Auth: Auth{Type: AuthNone}, RateLimit: RateLimit{Limit: 60, Window: 60}}
		}
		if t.API.Params == nil {
			t.API.Params = []Param{}
		}
		t.Record, t.Schedule = nil, nil
	case TriggerSchedule:
		if t.Schedule == nil {
			t.Schedule = &ScheduleTrigger{Repeat: RepeatDaily}
		}
		t.API, t.Record = nil, nil
		s.Async = []AsyncFlow{}
	default:
		if t.Record == nil {
			t.Record = &RecordTrigger{}
		}
		t.API, t.Schedule = nil, nil
		s.Async = []AsyncFlow{}
	}
	s.AllSteps(func(step *Step) {
		def := moduleDef(step.Type)
		if def == nil {
			return
		}
		if !def.Branches {
			step.Then, step.Else = nil, nil
		}
		if step.Config == nil {
			if impl, ok := moduleImpls[step.Type]; ok {
				step.Config = impl.newConfig()
			} else {
				step.Config = json.RawMessage(`{}`)
			}
		}
	})
}

// IsAPI reports whether the workflow is triggered by API requests.
func (s *Spec) IsAPI() bool {
	return s.Trigger.Type == TriggerAPI && s.Trigger.API != nil
}

// Params returns the parameters declared by the API trigger; it returns nil for other triggers.
func (s *Spec) Params() []Param {
	if !s.IsAPI() {
		return nil
	}
	return s.Trigger.API.Params
}

// WalkSteps visits every step in the flow depth-first, including branches.
func WalkSteps(steps []Step, fn func(step *Step)) {
	for i := range steps {
		fn(&steps[i])
		WalkSteps(steps[i].Then, fn)
		WalkSteps(steps[i].Else, fn)
	}
}

// AllSteps visits all steps of the main flow and the async flows.
func (s *Spec) AllSteps(fn func(step *Step)) {
	WalkSteps(s.Steps, fn)
	for i := range s.Async {
		WalkSteps(s.Async[i].Steps, fn)
	}
}

// WriteTables returns the UIDs of the tables the main flow writes to.
func (s *Spec) WriteTables() []string {
	var uids []string
	seen := map[string]bool{}
	WalkSteps(s.Steps, func(step *Step) {
		if uid := step.Table(); (step.Type == StepCreate || step.Type == StepUpdate) && uid != "" && !seen[uid] {
			seen[uid] = true
			uids = append(uids, uid)
		}
	})
	return uids
}

// Param returns the parameter with the given ID, or nil if none exists.
func (s *Spec) Param(id string) *Param {
	for i, p := range s.Params() {
		if p.ID == id {
			return &s.Trigger.API.Params[i]
		}
	}
	return nil
}
