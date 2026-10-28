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

// HubInstalled is the installation in Sayrud of a package of the catalog.
type HubInstalled struct {
	// Version is the installed version.
	Version string `json:"version"`
	// FromHub reports whether it is installed from Sayrud Hub; a module installed from elsewhere with the same id is
	// never replaced from the catalog.
	FromHub bool `json:"fromHub"`
	// UID identifies the installed field shortcut, empty for a workflow plugin, which is identified by its id.
	UID string `json:"uid,omitempty"`
	// Enabled reports whether the installed module can be used.
	Enabled bool `json:"enabled"`
} // @name HubInstalled

// HubPackage is a package of the Sayrud Hub catalog, its texts are in the language of the request.
type HubPackage struct {
	Kind        string `json:"kind" enums:"workflow_plugin,field_shortcut"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Icon is a Lucide icon name or a data:image URL, empty for the default icon.
	Icon  string `json:"icon"`
	Color string `json:"color"`
	// Category is data, logic or integration of a workflow plugin, empty for a field shortcut.
	Category string   `json:"category"`
	Author   string   `json:"author"`
	Homepage string   `json:"homepage"`
	Tags     []string `json:"tags"`
	// Official reports whether the package is published by the Sayrud team.
	Official bool `json:"official"`
	// Publisher is the user name of the Hub account publishing the package, empty if the account is removed.
	Publisher string `json:"publisher"`
	// LatestVersion is the version installed from the catalog, empty if no release is available.
	LatestVersion string     `json:"latestVersion"`
	Downloads     int64      `json:"downloads"`
	ReleasedAt    *time.Time `json:"releasedAt,omitempty"`
	// URL is the page of the package on Sayrud Hub.
	URL string `json:"url"`
	// Installed is omitted if the package is not installed.
	Installed *HubInstalled `json:"installed,omitempty"`
} // @name HubPackage

// ToHubPackage converts the package of the catalog, installed is nil if it is not installed.
func ToHubPackage(p *hub.CatalogPackage, installed *HubInstalled) *HubPackage {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	publisher := ""
	if p.Publisher != nil {
		publisher = p.Publisher.UserName
	}
	return &HubPackage{
		Kind: p.Kind, ID: p.ID, Name: p.Name, Description: p.Description, Icon: p.Icon, Color: p.Color,
		Category: p.Category, Author: p.Author, Homepage: p.Homepage, Tags: tags, Official: p.Official, Publisher: publisher,
		LatestVersion: p.LatestVersion, Downloads: p.Downloads, ReleasedAt: p.ReleasedAt, URL: hub.PageURL(p.Kind, p.ID),
		Installed: installed,
	}
}

// HubPackagesResp is a page of the Sayrud Hub catalog.
type HubPackagesResp struct {
	Packages []*HubPackage `json:"packages"`
	Total    int64         `json:"total"`
	// URL is the catalog page of Sayrud Hub listing the packages of the kind.
	URL string `json:"url"`
} // @name HubPackagesResp

// HubPermissions are what the source of a release asks for.
type HubPermissions struct {
	// Fetch are the hosts the module can request, including their subdomains; * allows any public host.
	Fetch []string `json:"fetch"`
	// AI reports whether a field shortcut calls the configured AI model.
	AI bool `json:"ai"`
	// Credentials are the labels of the credentials the module asks for.
	Credentials []string `json:"credentials"`
} // @name HubPermissions

// HubReleaseDetail is the latest release of a package of the catalog.
type HubReleaseDetail struct {
	Version  string `json:"version"`
	Language string `json:"language" enums:"javascript,typescript"`
	// Size is the length of the source in bytes.
	Size int64 `json:"size"`
	// Timeout is the execution limit in seconds.
	Timeout int `json:"timeout"`
	// ResultType is the type of the fields a field shortcut can be attached to, empty for a workflow plugin.
	ResultType  string         `json:"resultType,omitempty"`
	Permissions HubPermissions `json:"permissions"`
	// HasPreview reports whether a workflow plugin supports the simulated debugging.
	HasPreview bool `json:"hasPreview"`
	// MinAppVersion is the lowest Sayrud version the release works with, empty if not declared.
	MinAppVersion string `json:"minAppVersion"`
	// Compatible reports whether the release works with this Sayrud version.
	Compatible  bool       `json:"compatible"`
	Changelog   string     `json:"changelog"`
	Prerelease  bool       `json:"prerelease"`
	Yanked      bool       `json:"yanked"`
	YankReason  string     `json:"yankReason,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
} // @name HubReleaseDetail

// HubPackageDetail is a package of the Sayrud Hub catalog with its readme and latest release.
type HubPackageDetail struct {
	HubPackage
	// Readme is in Markdown.
	Readme string `json:"readme"`
	// Latest is omitted if no release is available.
	Latest *HubReleaseDetail `json:"latest,omitempty"`
} // @name HubPackageDetail

// ToHubPackageDetail converts the package of the catalog, appVersion tells whether its latest release is compatible.
func ToHubPackageDetail(p *hub.PackageDetail, installed *HubInstalled, appVersion string) *HubPackageDetail {
	resp := &HubPackageDetail{HubPackage: *ToHubPackage(p.CatalogPackage, installed), Readme: p.Readme}
	if r := p.Latest; r != nil {
		permissions := HubPermissions{Fetch: r.Permissions.Fetch, AI: r.Permissions.AI, Credentials: r.Permissions.Credentials}
		if permissions.Fetch == nil {
			permissions.Fetch = []string{}
		}
		if permissions.Credentials == nil {
			permissions.Credentials = []string{}
		}
		resp.Latest = &HubReleaseDetail{
			Version: r.Version, Language: r.Language, Size: r.Size, Timeout: r.Timeout, ResultType: r.ResultType,
			Permissions: permissions, HasPreview: r.HasPreview, MinAppVersion: r.MinAppVersion,
			Compatible: hub.Compatible(appVersion, r.MinAppVersion), Changelog: r.Changelog, Prerelease: r.Prerelease,
			Yanked: r.Yanked, YankReason: r.YankReason, PublishedAt: r.PublishedAt,
		}
	}
	return resp
}

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
