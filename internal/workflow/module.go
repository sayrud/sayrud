package workflow

import (
	"embed"
	"encoding/json"
	"path"
	"slices"
	"sort"

	"github.com/cockroachdb/errors"
)

//go:embed defs/modules/*.json defs/credentials/*.json
var defsFS embed.FS

// FieldDef is a field of a configuration form; texts such as label, help and placeholder are frontend i18n keys.
type FieldDef struct {
	// Key is the property name in the config object, empty for display-only widgets such as branches.
	Key string `json:"key,omitempty"`
	// Widget is value, number, select, text, email, password, credential, table, filters, sort, fieldMapping, conditions, headers, body, status or branches.
	Widget      string `json:"widget"`
	Label       string `json:"label,omitempty"`
	Help        string `json:"help,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Prefix      string `json:"prefix,omitempty"`
	Suffix      string `json:"suffix,omitempty"`
	// Display is the presentation of the widget, e.g. radio for select, response / optional for body.
	Display  string `json:"display,omitempty"`
	Required bool   `json:"required,omitempty"`
	// ShowIf is another field of the same config; this field is hidden while that one is empty.
	ShowIf    string          `json:"showIf,omitempty"`
	Min       *float64        `json:"min,omitempty"`
	Max       *float64        `json:"max,omitempty"`
	MaxLength int             `json:"maxLength,omitempty"`
	Multiline bool            `json:"multiline,omitempty"`
	Default   json.RawMessage `json:"default,omitempty" swaggertype:"object"`
	Options   []OptionDef     `json:"options,omitempty"`
	// Resets are the fields cleared when this field changes, e.g. the filters when the table changes.
	Resets []string `json:"resets,omitempty"`
	// CredentialType is the credential type a credential widget selects.
	CredentialType string `json:"credentialType,omitempty"`
	// Secret marks a credential field that is stored encrypted and never returned to the frontend.
	Secret bool `json:"secret,omitempty"`
} // @name WorkflowFieldDef

// OptionDef is an option of a select widget; Patch changes other fields along with the selection, only when they still hold the value of the previous option.
type OptionDef struct {
	Value json.RawMessage            `json:"value" swaggertype:"object"`
	Label string                     `json:"label,omitempty"`
	Patch map[string]json.RawMessage `json:"patch,omitempty" swaggertype:"object"`
} // @name WorkflowOptionDef

// SectionDef is a group of a configuration form.
type SectionDef struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ShowIf      string `json:"showIf,omitempty"`

	// ShowIfValue limits a credential section to this value of ShowIf.
	ShowIfValue string `json:"showIfValue,omitempty"`

	// Extra is a compact field shown on the right of the section title.
	Extra  *FieldDef  `json:"extra,omitempty"`
	Fields []FieldDef `json:"fields"`
} // @name WorkflowSectionDef

// OutputDef is an output of a step, referenced as steps.<id>.<key>.
type OutputDef struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Kind is string, number, boolean, object, array or record; record expands the fields of the table given by TableKey.
	Kind string `json:"kind"`
	// Items is the element type of an array, record for table records.
	Items string `json:"items,omitempty"`
} // @name WorkflowOutputDef

// SummaryDef describes the summary on the node card.
// Template and Args form an i18n message with args such as table:tableUID (the table name) and count:filters (the array length);
// otherwise Parts are joined, each like raw:method (as is), value:url (the value description) or i18n:<key>.
type SummaryDef struct {
	Template  string            `json:"template,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
	Parts     []string          `json:"parts,omitempty"`
	Separator string            `json:"separator,omitempty"`
	// Fallback is shown while the Requires field is empty.
	Requires string `json:"requires,omitempty"`
	Fallback string `json:"fallback,omitempty"`
} // @name WorkflowSummaryDef

// ModuleDef is the JSON DSL of a step module: the node appearance, allowed flows, default config, outputs and configuration form.
type ModuleDef struct {
	Type  string `json:"type"`
	Order int    `json:"order"`
	// Category groups the module in the node picker, one of ModuleCategories.
	Category    string `json:"category" enums:"data,logic,integration"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	// IDBase is the prefix of new step IDs, e.g. query, query2.
	IDBase string `json:"idBase"`
	// Flows are the allowed flows: sync (the API sync flow), async (API async flows) and background (the main flow of other triggers).
	Flows []string `json:"flows"`
	// Action marks external side effects; simulated debugging only computes the arguments.
	Action bool `json:"action,omitempty"`
	// Branches marks modules with then / else branches.
	Branches bool `json:"branches,omitempty"`
	// BranchLabels overrides the default then / else labels.
	BranchLabels map[string]string `json:"branchLabels,omitempty"`
	// Terminal marks modules that return the response and end the sync flow.
	Terminal bool `json:"terminal,omitempty"`
	// TableKey is the config field referencing a table.
	TableKey string          `json:"tableKey,omitempty"`
	Defaults json.RawMessage `json:"defaults" swaggertype:"object"`
	Outputs  []OutputDef     `json:"outputs"`
	Summary  *SummaryDef     `json:"summary,omitempty"`
	Sections []SectionDef    `json:"sections"`
	// Plugin is the installed plugin implementing the module, nil for built-in modules; texts of plugin modules are plain text rather than i18n keys.
	Plugin *ModulePlugin `json:"plugin,omitempty"`
} // @name WorkflowModuleDef

// ModulePlugin describes the installed plugin of a module.
type ModulePlugin struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Author   string `json:"author,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	// Enabled is false when the admin disabled the plugin: its steps are kept but can not be added, published or executed.
	Enabled bool `json:"enabled"`
} // @name WorkflowModulePlugin

// CredentialTestDef describes the test action of a credential type and its extra input.
type CredentialTestDef struct {
	Description string     `json:"description,omitempty"`
	Action      string     `json:"action"`
	Success     string     `json:"success"`
	Fields      []FieldDef `json:"fields"`
} // @name WorkflowCredentialTestDef

// CredentialTypeDef is the JSON DSL of a credential type.
type CredentialTypeDef struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon"`
	// Summary are the non-secret fields that tell credentials apart in lists.
	Summary  []string           `json:"summary"`
	Sections []SectionDef       `json:"sections"`
	Test     *CredentialTestDef `json:"test,omitempty"`
} // @name WorkflowCredentialTypeDef

// Fields returns the fields of all sections in order.
func (d *CredentialTypeDef) Fields() []FieldDef {
	var out []FieldDef
	for _, s := range d.Sections {
		out = append(out, s.Fields...)
	}
	return out
}

// FieldsFor returns the credential fields shown for the submitted data.
func (d *CredentialTypeDef) FieldsFor(data map[string]interface{}) []FieldDef {
	var out []FieldDef

	for _, s := range d.Sections {
		if s.ShowIfValue == "" || data[s.ShowIf] == s.ShowIfValue {
			out = append(out, s.Fields...)
		}
	}

	return out
}

// ModuleCategories are the categories of the node picker in display order.
var ModuleCategories = []string{"data", "logic", "integration"}

var (
	moduleDefs     = map[string]*ModuleDef{}
	moduleList     []*ModuleDef
	credentialDefs = map[string]*CredentialTypeDef{}
	credentialList []*CredentialTypeDef
)

func init() {
	if err := loadDefs(); err != nil {
		panic(err)
	}
}

func loadDefs() error {
	modules, err := defsFS.ReadDir("defs/modules")
	if err != nil {
		return err
	}
	for _, entry := range modules {
		raw, err := defsFS.ReadFile(path.Join("defs/modules", entry.Name()))
		if err != nil {
			return err
		}
		var def ModuleDef
		if err := json.Unmarshal(raw, &def); err != nil {
			return errors.Wrapf(err, "parse module %s", entry.Name())
		}
		if _, ok := moduleImpls[def.Type]; !ok {
			return errors.Newf("module %q has no implementation", def.Type)
		}
		if !slices.Contains(ModuleCategories, def.Category) {
			return errors.Newf("module %q has an unknown category %q", def.Type, def.Category)
		}
		moduleDefs[def.Type] = &def
		moduleList = append(moduleList, &def)
	}
	for t := range moduleImpls {
		if moduleDefs[t] == nil {
			return errors.Newf("module %q has no definition", t)
		}
	}
	sort.Slice(moduleList, func(i, j int) bool { return moduleList[i].Order < moduleList[j].Order })

	creds, err := defsFS.ReadDir("defs/credentials")
	if err != nil {
		return err
	}
	for _, entry := range creds {
		raw, err := defsFS.ReadFile(path.Join("defs/credentials", entry.Name()))
		if err != nil {
			return err
		}
		var def CredentialTypeDef
		if err := json.Unmarshal(raw, &def); err != nil {
			return errors.Wrapf(err, "parse credential type %s", entry.Name())
		}
		credentialDefs[def.Type] = &def
		credentialList = append(credentialList, &def)
	}
	sort.Slice(credentialList, func(i, j int) bool { return credentialList[i].Type < credentialList[j].Type })
	for t := range credentialTesters {
		if credentialDefs[t] == nil {
			return errors.Newf("credential type %q has no definition", t)
		}
	}
	return nil
}

// Modules returns the definitions of the built-in step modules in display order.
func Modules() []*ModuleDef {
	return moduleList
}

// CredentialType returns the definition of a built-in or plugin credential type in English, or nil if it does not exist.
func CredentialType(t string) *CredentialTypeDef {
	if def := credentialDefs[t]; def != nil {
		return def
	}
	if p := plugins.get(t); p != nil {
		return p.manifest.CredentialTypeDef("en")
	}
	return nil
}

// moduleImpl is the backend implementation of a module; generic field checks come from the DSL, check only covers what the DSL can not express.
type moduleImpl struct {
	newConfig func() interface{}
	check     func(c *checker, step *Step, scope *flowScope, target string) error
	exec      func(x *execution, step *Step) (interface{}, error)
	preview   func(x *execution, step *Step) (interface{}, error)
	// ownTx means exec opens its own transaction and saves the output in background flows.
	ownTx bool
}

var moduleImpls = map[string]moduleImpl{
	StepQuery: {
		newConfig: func() interface{} { return &QueryConfig{} },
		check:     (*checker).checkQuery,
		exec:      (*execution).query,
	},
	StepCreate: {
		newConfig: func() interface{} { return &WriteConfig{} },
		check:     (*checker).checkWrite,
		exec:      (*execution).write,
		ownTx:     true,
	},
	StepUpdate: {
		newConfig: func() interface{} { return &WriteConfig{} },
		check:     (*checker).checkWrite,
		exec:      (*execution).write,
		ownTx:     true,
	},
	StepCondition: {
		newConfig: func() interface{} { return &ConditionConfig{} },
		check:     (*checker).checkCondition,
	},
	StepResponse: {
		newConfig: func() interface{} { return &ResponseConfig{} },
	},
	StepEmail: {
		newConfig: func() interface{} { return &EmailConfig{} },
		exec:      (*execution).email,
		preview:   (*execution).previewEmail,
	},
	StepHTTP: {
		newConfig: func() interface{} { return &HTTPConfig{} },
		exec:      (*execution).fetch,
		preview:   (*execution).previewHTTP,
	},
}

// QueryConfig is the config of a query step.
type QueryConfig struct {
	TableUID string   `json:"tableUID"`
	Filters  []Filter `json:"filters,omitempty"`
	// Match combines the filters, all or any; empty is equivalent to all.
	Match string `json:"match,omitempty" enums:"all,any"`
	Sort  []Sort `json:"sort,omitempty"`
	// Limit is the maximum number of records, 1–100; 0 means the default.
	Limit int `json:"limit,omitempty"`
}

// WriteConfig is the config of create and update steps.
type WriteConfig struct {
	TableUID string `json:"tableUID"`
	// RecordUID is the record to update, only used by update.
	RecordUID *Value       `json:"recordUID,omitempty"`
	Fields    []FieldValue `json:"fields,omitempty"`
}

// ConditionConfig is the config of a condition step.
type ConditionConfig struct {
	Match      string      `json:"match,omitempty" enums:"all,any"`
	Conditions []Condition `json:"conditions,omitempty"`
}

// ResponseConfig is the config of a response step; the flow can not set response headers.
type ResponseConfig struct {
	Status int   `json:"status"`
	Body   *Body `json:"body,omitempty"`
}

// EmailConfig is the config of an email step.
type EmailConfig struct {
	// Credential is the UID of an smtp credential of the project.
	Credential string `json:"credential"`
	To         *Value `json:"to,omitempty"`
	CC         *Value `json:"cc,omitempty"`
	Subject    *Value `json:"subject,omitempty"`
	Content    *Value `json:"content,omitempty"`
}

// HTTPConfig is the config of an http step; Timeout is in seconds, 1–30.
type HTTPConfig struct {
	Method  string   `json:"method,omitempty"`
	URL     *Value   `json:"url,omitempty"`
	Headers []Header `json:"headers,omitempty"`
	Body    *Body    `json:"body,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

// configOf returns the typed config of the step, or the zero value on a type mismatch so a broken config can not panic.
func configOf[T any](step *Step) *T {
	if c, ok := step.Config.(*T); ok && c != nil {
		return c
	}
	return new(T)
}

func (s *Step) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID     string          `json:"id"`
		Type   string          `json:"type"`
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
		Then   []Step          `json:"then"`
		Else   []Step          `json:"else"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw.Config) == 0 || string(raw.Config) == "null" {
		raw.Config = json.RawMessage(`{}`)
	}
	*s = Step{ID: raw.ID, Type: raw.Type, Name: raw.Name, Then: raw.Then, Else: raw.Else}
	impl, ok := moduleImpls[raw.Type]
	if !ok {
		s.Config = raw.Config
		return nil
	}
	cfg := impl.newConfig()
	if len(raw.Config) > 0 && string(raw.Config) != "null" {
		if err := json.Unmarshal(raw.Config, cfg); err != nil {
			return errors.Wrapf(err, "step %s config", raw.ID)
		}
	}
	s.Config = cfg
	return nil
}

// Table returns the UID of the table the step operates on, or an empty string.
func (s *Step) Table() string {
	switch c := s.Config.(type) {
	case *QueryConfig:
		return c.TableUID
	case *WriteConfig:
		return c.TableUID
	}
	return ""
}

// rawConfig converts the config to raw JSON keyed by field name for the generic DSL checks.
func rawConfig(step *Step) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	raw, err := json.Marshal(step.Config)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]json.RawMessage{}
	}
	return out
}

func isEmptyJSON(raw json.RawMessage) bool {
	s := string(raw)
	return len(raw) == 0 || s == "null" || s == `""` || s == "[]" || s == "{}"
}
