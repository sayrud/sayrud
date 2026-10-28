package form

import "encoding/json"

// InspectWorkflowPlugin is a plugin source to check before installing; the source is downloaded from URL when Source is empty.
type InspectWorkflowPlugin struct {
	Source string `json:"source,omitempty"`
	// Language of Source, javascript by default.
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// URL is an http or https address of the source, a source URL of Sayrud Hub is verified by its signature.
	URL string `json:"url,omitempty"`
	// HubID downloads a release of the plugin from Sayrud Hub instead, it takes precedence over Source and URL.
	HubID string `json:"hubID,omitempty"`
	// HubVersion is the exact release of HubID to inspect, latest when omitted.
	HubVersion string `json:"hubVersion,omitempty"`
} // @name InspectWorkflowPlugin

// InstallWorkflowPlugin installs a plugin, or upgrades the installed plugin of the same manifest id.
type InstallWorkflowPlugin struct {
	Source string `json:"source"`
	// Language of Source, javascript by default.
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// SourceURL is the address the source was downloaded from, kept for reference.
	SourceURL string `json:"sourceURL,omitempty"`
	// Signature is the Sayrud Hub signature returned by the inspection, it must describe Source.
	Signature json.RawMessage `json:"signature,omitempty" swaggertype:"object"`
} // @name InstallWorkflowPlugin

// SetWorkflowPluginEnabled enables or disables a plugin.
type SetWorkflowPluginEnabled struct {
	Enabled bool `json:"enabled"`
} // @name SetWorkflowPluginEnabled

// TestWorkflowPlugin runs execute of a plugin with a config; the unsaved Source is run when given, otherwise the installed plugin of PluginID.
type TestWorkflowPlugin struct {
	PluginID string `json:"pluginID,omitempty"`
	Source   string `json:"source,omitempty"`
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// Config is passed to execute as is, so credential fields hold the credential values.
	Config map[string]interface{} `json:"config"`
} // @name TestWorkflowPlugin
