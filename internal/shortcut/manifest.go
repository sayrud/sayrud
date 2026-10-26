package shortcut

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/manifest"
)

const (
	// DefaultTimeout is the time limit of an execution in seconds when the manifest does not set one.
	DefaultTimeout = 30
	// MaxTimeout is the longest time limit of an execution in seconds.
	MaxTimeout     = 300
	maxCredentials = 20
)

var headerNamePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")

// Manifest is the manifest exported by a field shortcut module. The properties shared with the workflow plugins
// are named, typed and validated the same way, see the manifest package.
type Manifest struct {
	// ID identifies the source package. It is optional for local scripts and distinct from the installation UID.
	ID string `json:"id,omitempty"`
	// Name is shown in the field editor.
	Name i18n.LocalText `json:"name"`
	// Version is the version of the shortcut, e.g. 1.0.0.
	Version string `json:"version"`
	// Description tells the members what the shortcut does.
	Description i18n.LocalText `json:"description,omitempty"`
	// Homepage is the project website or support page.
	Homepage string `json:"homepage,omitempty"`
	// ResultType is the type of the fields the shortcut can be attached to.
	ResultType string `json:"resultType" enums:"text,single_select,multi_select,datetime,number,checkbox"`
	// Timeout is the time limit of an execution in seconds between 1 and 300, 0 defaults to 30.
	Timeout int `json:"timeout,omitempty"`
	// Permissions are the hosts the script can fetch and whether it can use the globally configured AI model.
	Permissions db.ShortcutPermissions `json:"permissions"`
	// Credentials are the credentials the script can use when fetching, without their values.
	Credentials []db.ShortcutCredential `json:"credentials"`
	// Sections are the form fields configured in the field editor and passed to execute as the params.
	Sections []db.ShortcutSection `json:"sections"`
} // @name ShortcutManifest

// renamedProperties are the properties of the shortcut manifests renamed to match the workflow plugins.
var renamedProperties = [][2]string{
	{"formItems", "sections"}, {"domains", "permissions.fetch"}, {"aiEnabled", "permissions.ai"}, {"timeoutSeconds", "timeout"},
}

// ParseManifest decodes and validates the manifest exported by a shortcut, filling in the defaults; errors are *manifest.Error.
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, manifest.Errorf("the manifest can not be decoded: %v", err)
	}
	if err := checkRenamed(raw); err != nil {
		return nil, err
	}
	if err := m.Normalize(); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkRenamed reports the properties named as before the manifest was unified, which would otherwise be ignored.
func checkRenamed(raw []byte) error {
	var properties map[string]json.RawMessage
	if json.Unmarshal(raw, &properties) != nil {
		return nil
	}
	for _, p := range renamedProperties {
		if _, ok := properties[p[0]]; ok {
			return manifest.Errorf("%s is renamed to %s", p[0], p[1])
		}
	}
	var sections []struct {
		Fields []map[string]json.RawMessage `json:"fields"`
	}
	if json.Unmarshal(properties["sections"], &sections) != nil {
		return nil
	}
	for _, s := range sections {
		for _, f := range s.Fields {
			if _, ok := f["component"]; ok {
				var key string
				_ = json.Unmarshal(f["key"], &key)
				return manifest.Errorf("field %q uses component, which is renamed to widget", key)
			}
		}
	}
	return nil
}

// Normalize validates the manifest and fills in the defaults; errors are *manifest.Error.
func (m *Manifest) Normalize() error {
	if m.ID != "" {
		if err := manifest.CheckID(m.ID); err != nil {
			return err
		}
	}
	if err := manifest.CheckHomepage(m.Homepage); err != nil {
		return err
	}
	if err := manifest.CheckText(m.Name, "name", true); err != nil {
		return err
	}
	if err := manifest.CheckVersion(m.Version); err != nil {
		return err
	}
	if err := manifest.CheckText(m.Description, "description", false); err != nil {
		return err
	}
	resultType := db.SLFieldType(m.ResultType)
	if !resultType.CanHostShortcut() {
		return manifest.Errorf("resultType must be one of %s", strings.Join(lo.Map(db.ShortcutHostFieldTypes, func(t db.SLFieldType, _ int) string { return string(t) }), ", "))
	}

	timeout, err := manifest.Timeout(m.Timeout, DefaultTimeout, MaxTimeout)
	if err != nil {
		return err
	}
	m.Timeout = timeout
	if m.Permissions.Fetch, err = manifest.NormalizeHosts(m.Permissions.Fetch); err != nil {
		return err
	}

	if err := m.normalizeCredentials(); err != nil {
		return err
	}
	return m.normalizeSections(resultType)
}

func (m *Manifest) normalizeCredentials() error {
	if len(m.Credentials) > maxCredentials {
		return manifest.Errorf("there are more than %d credentials", maxCredentials)
	}
	keys := map[string]bool{}
	for i := range m.Credentials {
		c := &m.Credentials[i]
		if err := manifest.CheckKey(keys, c.Key, "credential"); err != nil {
			return err
		}
		if err := manifest.CheckText(c.Label, fmt.Sprintf("the label of credential %q", c.Key), false); err != nil {
			return err
		}
		switch {
		case !c.Type.Valid():
			return manifest.Errorf("credential %q must have the type bearer, header or query", c.Key)
		case c.Type == db.ShortcutCredentialBearer:
			c.Name = ""
		case c.Name == "":
			return manifest.Errorf("credential %q must have the name of the header or query parameter", c.Key)
		case c.Type == db.ShortcutCredentialHeader && !headerNamePattern.MatchString(c.Name):
			return manifest.Errorf("credential %q has an invalid header name %q", c.Key, c.Name)
		}
	}
	if m.Credentials == nil {
		m.Credentials = []db.ShortcutCredential{}
	}
	return nil
}

var shortcutWidgets = []string{
	string(db.ShortcutWidgetFieldSelect), string(db.ShortcutWidgetText), string(db.ShortcutWidgetSelect),
	string(db.ShortcutWidgetFieldOptions), string(db.ShortcutWidgetPrompt),
}

func (m *Manifest) normalizeSections(resultType db.SLFieldType) error {
	if len(m.Sections) > manifest.MaxSections {
		return manifest.Errorf("there are more than %d sections", manifest.MaxSections)
	}
	keys := map[string]bool{}
	count := 0
	for i := range m.Sections {
		s := &m.Sections[i]
		if err := manifest.CheckText(s.Title, "a section title", false); err != nil {
			return err
		}
		if err := manifest.CheckText(s.Description, "a section description", false); err != nil {
			return err
		}
		for j := range s.Fields {
			count++
			f := &s.Fields[j]
			if !f.Widget.Valid() {
				return manifest.Errorf("field %q has an unsupported widget %q, use one of %s", f.Key, f.Widget, strings.Join(shortcutWidgets, ", "))
			}
			if err := normalizeField(f, keys); err != nil {
				return err
			}
			if f.Widget == db.ShortcutWidgetFieldOptions && resultType != db.SingleSelectFieldType && resultType != db.MultiSelectFieldType {
				return manifest.Errorf("field %q edits the field options, which needs the resultType single_select or multi_select", f.Key)
			}
		}
		if s.Fields == nil {
			s.Fields = []db.ShortcutField{}
		}
	}
	if count > manifest.MaxFields {
		return manifest.Errorf("there are more than %d fields", manifest.MaxFields)
	}
	if m.Sections == nil {
		m.Sections = []db.ShortcutSection{}
	}
	return nil
}

func normalizeField(f *db.ShortcutField, keys map[string]bool) error {
	if err := manifest.CheckKey(keys, f.Key, "field"); err != nil {
		return err
	}
	for _, t := range []struct {
		what string
		text i18n.LocalText
	}{{"label", f.Label}, {"help", f.Help}, {"placeholder", f.Placeholder}} {
		if err := manifest.CheckText(t.text, fmt.Sprintf("the %s of field %q", t.what, f.Key), false); err != nil {
			return err
		}
	}

	if f.Widget == db.ShortcutWidgetFieldSelect {
		for _, t := range f.FieldTypes {
			if !t.Check() || t == db.FormulaFieldType {
				return manifest.Errorf("field %q has an invalid field type %q in fieldTypes", f.Key, t)
			}
		}
		f.FieldTypes = lo.Uniq(f.FieldTypes)
	} else {
		f.FieldTypes = nil
	}

	if f.Widget == db.ShortcutWidgetSelect {
		if len(f.Options) == 0 || len(f.Options) > manifest.MaxOptions {
			return manifest.Errorf("field %q must have 1–%d options", f.Key, manifest.MaxOptions)
		}
		seen := map[string]bool{}
		for _, o := range f.Options {
			if o.Value == "" || seen[o.Value] {
				return manifest.Errorf("field %q has an invalid or duplicated option value", f.Key)
			}
			seen[o.Value] = true
			if err := manifest.CheckText(o.Label, fmt.Sprintf("an option label of field %q", f.Key), false); err != nil {
				return err
			}
		}
		if f.Default != "" && !seen[f.Default] {
			return manifest.Errorf("field %q has an invalid default", f.Key)
		}
	} else {
		f.Options = nil
	}

	if f.Widget != db.ShortcutWidgetText {
		f.Multiline = false
	}
	if f.Widget == db.ShortcutWidgetFieldSelect || f.Widget == db.ShortcutWidgetFieldOptions {
		f.Default = ""
	}
	return nil
}
