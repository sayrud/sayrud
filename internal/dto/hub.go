package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/hub"
)

// HubOrigin is the Sayrud Hub release a module is installed from, as asserted by its verified signature.
type HubOrigin struct {
	Version string `json:"version"`
	// Official reports whether the package is published by the Sayrud team.
	Official bool `json:"official"`
	// URL is the page of the package on Sayrud Hub.
	URL string `json:"url"`
	// Changelog is the release notes, only returned when inspecting a release.
	Changelog string `json:"changelog,omitempty"`
} // @name HubOrigin

// ToHubOrigin returns the origin asserted by the stored signature, nil if there is none or it is no longer trusted.
func ToHubOrigin(signature string) *HubOrigin {
	st := hub.Origin(signature)
	if st == nil {
		return nil
	}
	return &HubOrigin{Version: st.Version, Official: st.Official, URL: hub.PageURL(st.Kind, st.ID)}
}

// HubRelease is a release available on Sayrud Hub.
type HubRelease struct {
	Version string `json:"version"`
	// MinAppVersion is the lowest Sayrud version the release works with, empty if not declared.
	MinAppVersion string     `json:"minAppVersion"`
	Prerelease    bool       `json:"prerelease"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
} // @name HubRelease

// HubUpdate is the state on Sayrud Hub of an installed package.
type HubUpdate struct {
	Kind string `json:"kind" enums:"workflow_plugin,field_shortcut"`
	// ID is the package id, the plugin id or the manifest id of the shortcuts.
	ID string `json:"id"`
	// Version is the installed version.
	Version string `json:"version"`
	// URL is the page of the package on Sayrud Hub.
	URL string `json:"url"`
	// Latest is the newer release compatible with this Sayrud version, omitted if there is none.
	Latest *HubRelease `json:"latest,omitempty"`
	// Yanked reports whether the installed version has been withdrawn, it should be updated or uninstalled.
	Yanked     bool   `json:"yanked"`
	YankReason string `json:"yankReason,omitempty"`
	// Delisted reports whether the package has been removed from the catalog.
	Delisted     bool   `json:"delisted"`
	DelistReason string `json:"delistReason,omitempty"`
} // @name HubUpdate

// HubUpdatesResp lists the packages installed from Sayrud Hub that have an update or a notice.
type HubUpdatesResp struct {
	Packages []*HubUpdate `json:"packages"`
	// CheckedAt is the time the Hub was asked, the result is reused for a while.
	CheckedAt time.Time `json:"checkedAt"`
} // @name HubUpdatesResp

// ToHubUpdatesResp keeps the packages known by the Hub that have an update or a notice.
func ToHubUpdatesResp(u *hub.Updates) *HubUpdatesResp {
	resp := &HubUpdatesResp{Packages: []*HubUpdate{}, CheckedAt: u.CheckedAt}
	for _, p := range u.Packages {
		if !p.Found || (p.Latest == nil && !p.Yanked && !p.Delisted) {
			continue
		}
		update := &HubUpdate{
			Kind: p.Kind, ID: p.ID, Version: p.Version, URL: hub.PageURL(p.Kind, p.ID),
			Yanked: p.Yanked, YankReason: p.YankReason, Delisted: p.Delisted, DelistReason: p.DelistReason,
		}
		if p.Latest != nil {
			update.Latest = &HubRelease{Version: p.Latest.Version, MinAppVersion: p.Latest.MinAppVersion, Prerelease: p.Latest.Prerelease, PublishedAt: p.Latest.PublishedAt}
		}
		resp.Packages = append(resp.Packages, update)
	}
	return resp
}
