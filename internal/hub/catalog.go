package hub

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/mod/semver"
)

// Sorts of the catalog.
const (
	SortPopular = "popular"
	SortRecent  = "recent"
	SortNew     = "new"
)

// MaxPageSize is the largest page of the catalog the Hub serves.
const MaxPageSize = 100

// Publisher is the Hub account a package is published by.
type Publisher struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
}

// CatalogPackage is a package listed in the catalog, its texts are in the requested language.
type CatalogPackage struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Icon is a Lucide icon name or a data:image URL, empty for the default icon.
	Icon  string `json:"icon"`
	Color string `json:"color"`
	// Category is data, logic or integration of a workflow plugin, empty for a field shortcut.
	Category  string     `json:"category"`
	Author    string     `json:"author"`
	Homepage  string     `json:"homepage"`
	Tags      []string   `json:"tags"`
	Official  bool       `json:"official"`
	Publisher *Publisher `json:"publisher"`
	// LatestVersion is the version installed from the catalog, empty if no release is available.
	LatestVersion string     `json:"latestVersion"`
	Downloads     int64      `json:"downloads"`
	PublishedAt   *time.Time `json:"publishedAt"`
	ReleasedAt    *time.Time `json:"releasedAt"`
}

// Catalog is a page of the catalog.
type Catalog struct {
	Packages []*CatalogPackage `json:"packages"`
	Total    int64             `json:"total"`
}

// ListOptions filters and paginates the catalog, the zero values are the defaults of the Hub.
type ListOptions struct {
	Kind     string
	Keyword  string
	Category string
	Sort     string
	Page     int
	PageSize int
}

// ListPackages returns a page of the packages available on the Hub.
func ListPackages(ctx context.Context, opts ListOptions, lang string) (*Catalog, error) {
	query := url.Values{}
	for key, value := range map[string]string{"kind": opts.Kind, "q": opts.Keyword, "category": opts.Category, "sort": opts.Sort} {
		if value != "" {
			query.Set(key, value)
		}
	}
	if opts.Page > 0 {
		query.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", strconv.Itoa(min(opts.PageSize, MaxPageSize)))
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var c Catalog
	if err := call(ctx, http.MethodGet, "/open/v1/packages?"+query.Encode(), lang, nil, &c); err != nil {
		return nil, err
	}
	if c.Packages == nil {
		c.Packages = []*CatalogPackage{}
	}
	return &c, nil
}

// ReleasePermissions are what the source of a release asks for.
type ReleasePermissions struct {
	// Fetch are the hosts the module can request, including their subdomains; * allows any public host.
	Fetch []string `json:"fetch"`
	// AI reports whether a field shortcut calls the AI model.
	AI bool `json:"ai"`
	// Credentials are the labels of the credentials the module asks for.
	Credentials []string `json:"credentials"`
}

// ReleaseDetail is the latest release of a package as described in the catalog.
type ReleaseDetail struct {
	Version  string `json:"version"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	// Timeout is the execution limit in seconds.
	Timeout int `json:"timeout"`
	// ResultType is the type of the fields a field shortcut can be attached to, empty for a workflow plugin.
	ResultType    string             `json:"resultType"`
	Permissions   ReleasePermissions `json:"permissions"`
	HasPreview    bool               `json:"hasPreview"`
	MinAppVersion string             `json:"minAppVersion"`
	Changelog     string             `json:"changelog"`
	Prerelease    bool               `json:"prerelease"`
	Yanked        bool               `json:"yanked"`
	YankReason    string             `json:"yankReason"`
	Downloads     int64              `json:"downloads"`
	PublishedAt   *time.Time         `json:"publishedAt"`
}

// PackageDetail is a package of the catalog with its readme and latest release.
type PackageDetail struct {
	*CatalogPackage
	// Readme is in Markdown.
	Readme string `json:"readme"`
	// Latest is nil if no release is available.
	Latest *ReleaseDetail `json:"latest"`
}

// GetPackage returns the package of the catalog.
func GetPackage(ctx context.Context, kind, id, lang string) (*PackageDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var p PackageDetail
	if err := call(ctx, http.MethodGet, "/open/v1/packages/"+kindPath(kind)+"/"+url.PathEscape(id), lang, nil, &p); err != nil {
		return nil, err
	}
	if p.CatalogPackage == nil || p.Kind != kind || p.ID != id {
		return nil, &Error{Message: "the Hub returned another package"}
	}
	return &p, nil
}

// Compatible reports whether a release requiring the minimum version works with the app version. As on the Hub,
// an empty minimum or an app version that is not a release, such as a development build, is always compatible.
func Compatible(appVersion, minimum string) bool {
	if minimum == "" || !semver.IsValid("v"+appVersion) {
		return true
	}
	return semver.Compare("v"+appVersion, "v"+minimum) >= 0
}
