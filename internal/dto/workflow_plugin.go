package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow"
)

// WorkflowPlugin is an installed workflow node plugin, with the texts in the language of the request.
type WorkflowPlugin struct {
	// ID is the id of the manifest; the steps of the node have the type plugin:<id>.
	ID      string `json:"id"`
	Version string `json:"version"`
	// Language is the source language, javascript or typescript.
	Language string `json:"language" enums:"javascript,typescript"`
	// SourceURL is the address the source was downloaded from, empty if uploaded or written by hand.
	SourceURL string `json:"sourceURL"`
	Checksum  string `json:"checksum"`
	Enabled   bool   `json:"enabled"`
	// Module is the node DSL rendered by the workflow editor.
	Module *workflow.ModuleDef `json:"module"`
	// Credential is the credential type defined by the plugin, nil if there is none.
	Credential *workflow.CredentialTypeDef `json:"credential,omitempty"`
	// Domains are the hosts the node can fetch, including their subdomains; * allows any public host.
	Domains []string `json:"domains"`
	// Timeout is the time limit of a call in seconds.
	Timeout int `json:"timeout"`
	// UsedBy is the number of workflows whose draft or published revision uses the node.
	UsedBy int64 `json:"usedBy"`
	// Source is the source code, only returned when getting a single plugin.
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
} // @name WorkflowPlugin

// ToWorkflowPlugin converts an installed plugin with its validated manifest.
func ToWorkflowPlugin(p *db.WorkflowPlugin, m *workflow.PluginManifest, lang string, usedBy int64) *WorkflowPlugin {
	domains := m.Permissions.Fetch
	if domains == nil {
		domains = []string{}
	}
	return &WorkflowPlugin{
		ID: p.PluginID, Version: m.Version, Language: p.Language, SourceURL: p.SourceURL, Checksum: p.Checksum, Enabled: p.Enabled,
		Module: m.ModuleDef(lang, 0, p.Enabled), Credential: m.CredentialTypeDef(lang), Domains: domains, Timeout: m.Timeout,
		UsedBy: usedBy, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// ListWorkflowPluginsResp lists the installed plugins in installation order and the built-in nodes.
type ListWorkflowPluginsResp struct {
	Plugins []*WorkflowPlugin `json:"plugins"`
	// Builtins are the built-in nodes, whose texts are frontend i18n keys.
	Builtins []*workflow.ModuleDef `json:"builtins"`
} // @name ListWorkflowPluginsResp

// InspectWorkflowPluginResp is a plugin source checked before installing.
type InspectWorkflowPluginResp struct {
	// Plugin is the plugin as it would be installed.
	Plugin *WorkflowPlugin `json:"plugin"`
	// Installed is the plugin of the same id that would be replaced, nil if it is not installed.
	Installed *WorkflowPlugin `json:"installed,omitempty"`
	// Source is the checked source, downloaded when inspecting a URL.
	Source   string `json:"source"`
	Language string `json:"language" enums:"javascript,typescript"`
	// HasPreview reports whether the plugin exports preview for simulated debugging.
	HasPreview bool `json:"hasPreview"`
} // @name InspectWorkflowPluginResp

// TestWorkflowPluginResp is the result of a test call of a plugin.
type TestWorkflowPluginResp struct {
	// Value is the value returned by the hook.
	Value interface{} `json:"value,omitempty" swaggertype:"object"`
	// Error is the reason of the failure, empty if succeeded.
	Error string `json:"error,omitempty"`
	// Logs are the lines printed by console and ctx.log.
	Logs []string `json:"logs"`
	// Fetches is the number of the requests made by ctx.fetch.
	Fetches    int   `json:"fetches"`
	DurationMs int64 `json:"durationMs"`
} // @name TestWorkflowPluginResp
