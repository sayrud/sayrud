package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wuhan005/sayrud/internal/shortcut/script"
)

// PluginTypePrefix prefixes the step type and the credential type of an installed plugin, e.g. plugin:slack.
const PluginTypePrefix = "plugin:"

// PluginType returns the step type of the plugin with the manifest ID.
func PluginType(id string) string {
	return PluginTypePrefix + id
}

// IsPluginType reports whether the step or credential type belongs to a plugin.
func IsPluginType(t string) bool {
	return strings.HasPrefix(t, PluginTypePrefix)
}

// LocalText is a text of a plugin manifest, given as a string or as the translations keyed by language such as en and zh-CN.
type LocalText map[string]string

func (t *LocalText) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*t = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*t = LocalText{"": s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("a text must be a string or an object of translations")
	}
	*t = m
	return nil
}

// In returns the text in the language, falling back to the same base language, English, the untranslated string, then any translation.
func (t LocalText) In(lang string) string {
	if len(t) == 0 {
		return ""
	}
	if s, ok := t[lang]; ok {
		return s
	}
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if base, _, _ := strings.Cut(lang, "-"); base != "" {
		for _, k := range keys {
			if kb, _, _ := strings.Cut(k, "-"); strings.EqualFold(kb, base) {
				return t[k]
			}
		}
	}
	for _, k := range []string{"en", ""} {
		if s, ok := t[k]; ok {
			return s
		}
	}
	return t[keys[0]]
}

// PluginManifest is the manifest exported by a plugin module; it describes the node with the same DSL as the built-in modules.
type PluginManifest struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	Name        LocalText `json:"name"`
	Description LocalText `json:"description"`
	Author      string    `json:"author"`
	Homepage    string    `json:"homepage"`
	// Icon is a Lucide icon name such as message-square, or a data:image URL.
	Icon     string   `json:"icon"`
	Color    string   `json:"color"`
	Category string   `json:"category"`
	IDBase   string   `json:"idBase"`
	Flows    []string `json:"flows"`
	// Action marks external side effects: only these nodes can fetch, and simulated debugging calls preview instead of execute.
	Action bool `json:"action"`
	// Timeout is the time limit of a call in seconds.
	Timeout     int               `json:"timeout"`
	Permissions PluginPermissions `json:"permissions"`
	Credential  *PluginCredential `json:"credential"`
	Defaults    json.RawMessage   `json:"defaults"`
	Outputs     []PluginOutput    `json:"outputs"`
	Summary     *PluginSummary    `json:"summary"`
	Sections    []PluginSection   `json:"sections"`
}

// PluginPermissions are what the plugin is allowed to reach.
type PluginPermissions struct {
	// Fetch are the hosts ctx.fetch can request, including their subdomains; * allows any public host.
	Fetch []string `json:"fetch"`
}

// PluginCredential defines the credential type of the plugin, whose type is the step type of the plugin.
type PluginCredential struct {
	Label       LocalText       `json:"label"`
	Description LocalText       `json:"description"`
	Icon        string          `json:"icon"`
	Summary     []string        `json:"summary"`
	Sections    []PluginSection `json:"sections"`
}

type PluginOutput struct {
	Key   string    `json:"key"`
	Label LocalText `json:"label"`
	Kind  string    `json:"kind"`
	Items string    `json:"items"`
}

// PluginSummary is the summary on the node card, made of parts like raw:<key> and value:<key>.
type PluginSummary struct {
	Parts     []string  `json:"parts"`
	Separator string    `json:"separator"`
	Requires  string    `json:"requires"`
	Fallback  LocalText `json:"fallback"`
}

type PluginSection struct {
	Title       LocalText     `json:"title"`
	Description LocalText     `json:"description"`
	ShowIf      string        `json:"showIf"`
	Fields      []PluginField `json:"fields"`
}

type PluginField struct {
	Key            string          `json:"key"`
	Widget         string          `json:"widget"`
	Label          LocalText       `json:"label"`
	Help           LocalText       `json:"help"`
	Placeholder    LocalText       `json:"placeholder"`
	Prefix         LocalText       `json:"prefix"`
	Suffix         LocalText       `json:"suffix"`
	Display        string          `json:"display"`
	Required       bool            `json:"required"`
	ShowIf         string          `json:"showIf"`
	Min            *float64        `json:"min"`
	Max            *float64        `json:"max"`
	MaxLength      int             `json:"maxLength"`
	Multiline      bool            `json:"multiline"`
	Default        json.RawMessage `json:"default"`
	Options        []PluginOption  `json:"options"`
	CredentialType string          `json:"credentialType"`
	Secret         bool            `json:"secret"`
}

type PluginOption struct {
	Value json.RawMessage `json:"value"`
	Label LocalText       `json:"label"`
}

// ManifestError is an invalid manifest; Message is shown to the admin as is.
type ManifestError struct {
	Message string
}

func (e *ManifestError) Error() string {
	return e.Message
}

func manifestErrorf(format string, args ...interface{}) error {
	return &ManifestError{Message: fmt.Sprintf(format, args...)}
}

// Widgets a plugin can use in the step form and in the credential form.
var (
	pluginStepWidgets       = []string{"value", "number", "select", "credential", "headers", "body"}
	pluginCredentialWidgets = []string{"text", "email", "password", "number", "select"}
	pluginOutputKinds       = []string{"string", "number", "boolean", "object", "array"}
)

const (
	maxPluginSections = 20
	maxPluginFields   = 60
	maxPluginOutputs  = 50
	maxPluginOptions  = 100
	maxPluginText     = 500
	maxPluginDomains  = 50
	maxPluginIcon     = 100 << 10
	maxPluginTimeout  = 60
	// defaultPluginTimeout is the time limit of a call in seconds when the manifest does not set one.
	defaultPluginTimeout = 10
	defaultPluginColor   = "#6b4ff6"
	defaultPluginIcon    = "puzzle"
)

var (
	pluginIDPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{1,39}$`)
	pluginVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+\-]{0,31}$`)
	pluginKeyPattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	pluginIconPattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	pluginDataIconRegexp = regexp.MustCompile(`^data:image/(svg\+xml|png|jpeg|webp);base64,[A-Za-z0-9+/=]+$`)
	pluginColorPattern   = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	pluginLangPattern    = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

// ParsePluginManifest decodes and validates the manifest exported by a plugin, filling in the defaults; errors are *ManifestError.
func ParsePluginManifest(raw []byte) (*PluginManifest, error) {
	var m PluginManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, manifestErrorf("the manifest can not be decoded: %v", err)
	}
	if err := m.normalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

func checkText(t LocalText, what string, required bool) error {
	if required && strings.TrimSpace(t.In("en")) == "" {
		return manifestErrorf("%s is required", what)
	}
	if len(t) > 20 {
		return manifestErrorf("%s has too many translations", what)
	}
	for lang, s := range t {
		if lang != "" && !pluginLangPattern.MatchString(lang) {
			return manifestErrorf("%s has an invalid language %q", what, lang)
		}
		if utf8.RuneCountInString(s) > maxPluginText {
			return manifestErrorf("%s is longer than %d characters", what, maxPluginText)
		}
	}
	return nil
}

func (m *PluginManifest) normalize() error {
	switch {
	case !pluginIDPattern.MatchString(m.ID):
		return manifestErrorf("id must be 2–40 lowercase letters, digits or underscores starting with a letter")
	case !pluginVersionPattern.MatchString(m.Version):
		return manifestErrorf("version is required, e.g. 1.0.0")
	case utf8.RuneCountInString(m.Author) > 100:
		return manifestErrorf("author is longer than 100 characters")
	}
	if err := checkText(m.Name, "name", true); err != nil {
		return err
	}
	if err := checkText(m.Description, "description", false); err != nil {
		return err
	}
	if m.Homepage != "" {
		u, err := url.Parse(m.Homepage)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(m.Homepage) > 500 {
			return manifestErrorf("homepage must be an http or https URL")
		}
	}
	if err := checkIcon(&m.Icon); err != nil {
		return err
	}
	if m.Color == "" {
		m.Color = defaultPluginColor
	} else if !pluginColorPattern.MatchString(m.Color) {
		return manifestErrorf("color must be a hex color like #3370ff")
	}
	if m.Category == "" {
		m.Category = "integration"
	} else if !slices.Contains(ModuleCategories, m.Category) {
		return manifestErrorf("category must be one of %s", strings.Join(ModuleCategories, ", "))
	}
	if m.IDBase == "" {
		m.IDBase = m.ID
		if len(m.IDBase) > 24 {
			m.IDBase = m.IDBase[:24]
		}
	} else if !stepIDPattern.MatchString(m.IDBase) || len(m.IDBase) > 24 {
		return manifestErrorf("idBase must be at most 24 letters, digits or underscores starting with a letter")
	}

	if len(m.Flows) == 0 {
		m.Flows = []string{"sync", "async", "background"}
	}
	flows := make([]string, 0, len(m.Flows))
	for _, f := range []string{"sync", "async", "background"} {
		if slices.Contains(m.Flows, f) {
			flows = append(flows, f)
		}
	}
	for _, f := range m.Flows {
		if !slices.Contains(flows, f) {
			return manifestErrorf("flows must be sync, async or background")
		}
	}
	m.Flows = flows

	if m.Timeout == 0 {
		m.Timeout = defaultPluginTimeout
	} else if m.Timeout < 1 || m.Timeout > maxPluginTimeout {
		return manifestErrorf("timeout must be 1–%d seconds", maxPluginTimeout)
	}

	if len(m.Permissions.Fetch) > maxPluginDomains {
		return manifestErrorf("permissions.fetch has more than %d hosts", maxPluginDomains)
	}
	if len(m.Permissions.Fetch) > 0 && !m.Action {
		return manifestErrorf("only an action node can fetch, set action to true")
	}
	domains := make([]string, 0, len(m.Permissions.Fetch))
	for _, d := range m.Permissions.Fetch {
		if d == "*" {
			domains = append(domains, d)
			continue
		}
		normalized, ok := script.NormalizeDomain(strings.TrimPrefix(d, "*."))
		if !ok {
			return manifestErrorf("permissions.fetch has an invalid host %q", d)
		}
		domains = append(domains, normalized)
	}
	slices.Sort(domains)
	m.Permissions.Fetch = slices.Compact(domains)

	if m.Credential != nil {
		if err := m.Credential.normalize(); err != nil {
			return err
		}
	}

	if len(m.Sections) > maxPluginSections {
		return manifestErrorf("there are more than %d sections", maxPluginSections)
	}
	keys := map[string]bool{}
	count := 0
	for i := range m.Sections {
		s := &m.Sections[i]
		if err := checkText(s.Title, "a section title", false); err != nil {
			return err
		}
		if err := checkText(s.Description, "a section description", false); err != nil {
			return err
		}
		for j := range s.Fields {
			count++
			f := &s.Fields[j]
			if !slices.Contains(pluginStepWidgets, f.Widget) {
				return manifestErrorf("field %q has an unsupported widget %q, use one of %s", f.Key, f.Widget, strings.Join(pluginStepWidgets, ", "))
			}
			if err := f.normalize(keys); err != nil {
				return err
			}
			if f.Widget == "credential" {
				if f.CredentialType == "" {
					f.CredentialType = PluginType(m.ID)
				}
				own := f.CredentialType == PluginType(m.ID)
				if own && m.Credential == nil {
					return manifestErrorf("field %q uses the credential of the plugin, which is not defined", f.Key)
				}
				if !own && credentialDefs[f.CredentialType] == nil {
					return manifestErrorf("field %q uses an unknown credential type %q", f.Key, f.CredentialType)
				}
			}
		}
	}
	if count > maxPluginFields {
		return manifestErrorf("there are more than %d fields", maxPluginFields)
	}
	for _, s := range m.Sections {
		if s.ShowIf != "" && !keys[s.ShowIf] {
			return manifestErrorf("showIf refers to an unknown field %q", s.ShowIf)
		}
		for _, f := range s.Fields {
			if f.ShowIf != "" && !keys[f.ShowIf] {
				return manifestErrorf("showIf refers to an unknown field %q", f.ShowIf)
			}
		}
	}

	if len(m.Outputs) > maxPluginOutputs {
		return manifestErrorf("there are more than %d outputs", maxPluginOutputs)
	}
	outputs := map[string]bool{}
	for i := range m.Outputs {
		o := &m.Outputs[i]
		switch {
		case !pluginKeyPattern.MatchString(o.Key):
			return manifestErrorf("output %q has an invalid key", o.Key)
		case outputs[o.Key]:
			return manifestErrorf("output %q is duplicated", o.Key)
		case !slices.Contains(pluginOutputKinds, o.Kind):
			return manifestErrorf("output %q has an invalid kind, use one of %s", o.Key, strings.Join(pluginOutputKinds, ", "))
		case o.Items != "" && (o.Kind != "array" || !slices.Contains(pluginOutputKinds, o.Items)):
			return manifestErrorf("output %q has invalid items", o.Key)
		}
		if err := checkText(o.Label, fmt.Sprintf("the label of output %q", o.Key), false); err != nil {
			return err
		}
		outputs[o.Key] = true
	}

	if s := m.Summary; s != nil {
		for _, p := range s.Parts {
			kind, key, _ := strings.Cut(p, ":")
			if (kind != "raw" && kind != "value") || !keys[key] {
				return manifestErrorf("summary part %q must be raw:<field> or value:<field>", p)
			}
		}
		if s.Requires != "" && !keys[s.Requires] {
			return manifestErrorf("summary.requires refers to an unknown field %q", s.Requires)
		}
		if len(s.Separator) > 8 {
			return manifestErrorf("summary.separator is too long")
		}
		if err := checkText(s.Fallback, "summary.fallback", false); err != nil {
			return err
		}
	}

	defaults := map[string]json.RawMessage{}
	if len(m.Defaults) > 0 && string(m.Defaults) != "null" {
		if err := json.Unmarshal(m.Defaults, &defaults); err != nil {
			return manifestErrorf("defaults must be an object")
		}
	}
	for _, s := range m.Sections {
		for _, f := range s.Fields {
			if _, ok := defaults[f.Key]; !ok && len(f.Default) > 0 {
				defaults[f.Key] = f.Default
			}
		}
	}
	m.Defaults, _ = json.Marshal(defaults)
	return nil
}

func checkIcon(icon *string) error {
	switch {
	case *icon == "":
		*icon = defaultPluginIcon
	case strings.HasPrefix(*icon, "data:"):
		if len(*icon) > maxPluginIcon || !pluginDataIconRegexp.MatchString(*icon) {
			return manifestErrorf("icon must be a Lucide icon name or a base64 data:image URL of at most 100 KB")
		}
	case len(*icon) > 40 || !pluginIconPattern.MatchString(*icon):
		return manifestErrorf("icon must be a Lucide icon name such as message-square")
	}
	return nil
}

func (f *PluginField) normalize(keys map[string]bool) error {
	switch {
	case !pluginKeyPattern.MatchString(f.Key):
		return manifestErrorf("field %q has an invalid key", f.Key)
	case keys[f.Key]:
		return manifestErrorf("field %q is duplicated", f.Key)
	case f.Min != nil && f.Max != nil && *f.Min > *f.Max:
		return manifestErrorf("field %q has min greater than max", f.Key)
	case f.MaxLength < 0:
		return manifestErrorf("field %q has a negative maxLength", f.Key)
	case len(f.Default) > 0 && !json.Valid(f.Default):
		return manifestErrorf("field %q has an invalid default", f.Key)
	}
	keys[f.Key] = true
	for what, t := range map[string]LocalText{"label": f.Label, "help": f.Help, "placeholder": f.Placeholder, "prefix": f.Prefix, "suffix": f.Suffix} {
		if err := checkText(t, fmt.Sprintf("the %s of field %q", what, f.Key), false); err != nil {
			return err
		}
	}
	if f.Widget == "select" {
		if len(f.Options) == 0 || len(f.Options) > maxPluginOptions {
			return manifestErrorf("field %q must have 1–%d options", f.Key, maxPluginOptions)
		}
		seen := map[string]bool{}
		for _, o := range f.Options {
			if len(o.Value) == 0 || !json.Valid(o.Value) || seen[string(o.Value)] {
				return manifestErrorf("field %q has an invalid or duplicated option value", f.Key)
			}
			seen[string(o.Value)] = true
			if err := checkText(o.Label, fmt.Sprintf("an option label of field %q", f.Key), false); err != nil {
				return err
			}
		}
	} else {
		f.Options = nil
	}
	if f.Widget != "credential" {
		f.CredentialType = ""
	}
	return nil
}

func (c *PluginCredential) normalize() error {
	if err := checkText(c.Label, "credential.label", true); err != nil {
		return err
	}
	if err := checkText(c.Description, "credential.description", false); err != nil {
		return err
	}
	if c.Icon == "" {
		c.Icon = "key"
	} else if err := checkIcon(&c.Icon); err != nil {
		return err
	}
	if len(c.Sections) == 0 || len(c.Sections) > maxPluginSections {
		return manifestErrorf("credential must have 1–%d sections", maxPluginSections)
	}
	keys := map[string]bool{}
	secrets := map[string]bool{}
	for i := range c.Sections {
		s := &c.Sections[i]
		if err := checkText(s.Title, "a credential section title", false); err != nil {
			return err
		}
		s.Description, s.ShowIf = nil, ""
		for j := range s.Fields {
			f := &s.Fields[j]
			if !slices.Contains(pluginCredentialWidgets, f.Widget) {
				return manifestErrorf("credential field %q has an unsupported widget %q, use one of %s", f.Key, f.Widget, strings.Join(pluginCredentialWidgets, ", "))
			}
			if f.Widget == "password" {
				f.Secret = true
			}
			if err := f.normalize(keys); err != nil {
				return err
			}
			f.ShowIf = ""
			secrets[f.Key] = f.Secret
		}
	}
	if len(keys) == 0 {
		return manifestErrorf("credential must have at least one field")
	}
	for _, k := range c.Summary {
		if !keys[k] || secrets[k] {
			return manifestErrorf("credential.summary must list fields that are not secret")
		}
	}
	return nil
}

// ModuleDef converts the manifest into the module DSL in the language.
func (m *PluginManifest) ModuleDef(lang string, order int, enabled bool) *ModuleDef {
	def := &ModuleDef{
		Type: PluginType(m.ID), Order: order, Category: m.Category,
		Label: m.Name.In(lang), Description: m.Description.In(lang), Icon: m.Icon, Color: m.Color, IDBase: m.IDBase,
		Flows: m.Flows, Action: m.Action, Defaults: m.Defaults, Outputs: []OutputDef{}, Sections: pluginSections(m.Sections, lang),
		Plugin: &ModulePlugin{ID: m.ID, Version: m.Version, Author: m.Author, Homepage: m.Homepage, Enabled: enabled},
	}
	for _, o := range m.Outputs {
		label := o.Label.In(lang)
		if label == "" {
			label = o.Key
		}
		def.Outputs = append(def.Outputs, OutputDef{Key: o.Key, Label: label, Kind: o.Kind, Items: o.Items})
	}
	if s := m.Summary; s != nil {
		def.Summary = &SummaryDef{Parts: s.Parts, Separator: s.Separator, Requires: s.Requires, Fallback: s.Fallback.In(lang)}
	}
	return def
}

// CredentialTypeDef converts the credential of the manifest into the credential type DSL in the language, nil if there is none.
func (m *PluginManifest) CredentialTypeDef(lang string) *CredentialTypeDef {
	c := m.Credential
	if c == nil {
		return nil
	}
	summary := c.Summary
	if summary == nil {
		summary = []string{}
	}
	return &CredentialTypeDef{
		Type: PluginType(m.ID), Label: c.Label.In(lang), Description: c.Description.In(lang), Icon: c.Icon,
		Summary: summary, Sections: pluginSections(c.Sections, lang),
	}
}

func pluginSections(sections []PluginSection, lang string) []SectionDef {
	out := make([]SectionDef, 0, len(sections))
	for _, s := range sections {
		fields := make([]FieldDef, 0, len(s.Fields))
		for _, f := range s.Fields {
			fd := FieldDef{
				Key: f.Key, Widget: f.Widget, Label: f.Label.In(lang), Help: f.Help.In(lang), Placeholder: f.Placeholder.In(lang),
				Prefix: f.Prefix.In(lang), Suffix: f.Suffix.In(lang), Display: f.Display, Required: f.Required, ShowIf: f.ShowIf,
				Min: f.Min, Max: f.Max, MaxLength: f.MaxLength, Multiline: f.Multiline, Default: f.Default,
				CredentialType: f.CredentialType, Secret: f.Secret,
			}
			if fd.Widget == "password" {
				fd.Widget = "text"
			}
			for _, o := range f.Options {
				fd.Options = append(fd.Options, OptionDef{Value: o.Value, Label: o.Label.In(lang)})
			}
			fields = append(fields, fd)
		}
		out = append(out, SectionDef{Title: s.Title.In(lang), Description: s.Description.In(lang), ShowIf: s.ShowIf, Fields: fields})
	}
	return out
}
