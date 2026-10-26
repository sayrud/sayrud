package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	requestTimeout = 15 * time.Second
	// maxResponseSize bounds the JSON responses of the Hub.
	maxResponseSize = 4 << 20
	// maxUpdatePackages is the most packages the Hub checks per request.
	maxUpdatePackages = 200
	// updatesTTL is how long the result of an update check is reused.
	updatesTTL = time.Hour
)

var httpClient = &http.Client{Timeout: requestTimeout}

// ErrNotFound means the package or the release is not available on the Hub.
var ErrNotFound = errors.New("hub: package or release not found")

// Error is a failed request to the Hub; Message can be shown to the admin.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// VerifyError is a release whose signature does not describe the downloaded source; Err tells why.
type VerifyError struct {
	Err error
}

func (e *VerifyError) Error() string {
	return e.Err.Error()
}

func (e *VerifyError) Unwrap() error {
	return e.Err
}

// apiResponse is the envelope of the Hub API responses.
type apiResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// call requests the Hub API and decodes the data of a successful response into out.
func call(ctx context.Context, method, path, lang string, body, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.Wrap(err, "encode request")
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return errors.Wrap(err, "new request")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return requestError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return requestError(err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	var decoded apiResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return &Error{Message: fmt.Sprintf("unexpected response (%d)", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		if decoded.Msg != "" {
			return &Error{Message: decoded.Msg}
		}
		return &Error{Message: fmt.Sprintf("the server responded %d", resp.StatusCode)}
	}
	if err := json.Unmarshal(decoded.Data, out); err != nil {
		return &Error{Message: "unexpected response"}
	}
	return nil
}

func requestError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Message: "the request timed out"}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return &Error{Message: urlErr.Err.Error()}
	}
	return &Error{Message: err.Error()}
}

// Release is an approved release with what is needed to verify and install it.
type Release struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Version  string `json:"version"`
	Language string `json:"language"`
	// SourceURL downloads the source, which ends with the extension of the language.
	SourceURL string `json:"sourceURL"`
	// Signature is the DSSE envelope of the release statement.
	Signature     json.RawMessage `json:"signature"`
	MinAppVersion string          `json:"minAppVersion"`
	Changelog     string          `json:"changelog"`
	Official      bool            `json:"official"`
	PublishedAt   *time.Time      `json:"publishedAt"`
}

// GetRelease returns the release of the package, version is an exact version or LatestVersion.
func GetRelease(ctx context.Context, kind, id, version, lang string) (*Release, error) {
	var r Release
	path := "/open/v1/packages/" + kindPath(kind) + "/" + url.PathEscape(id) + "/versions/" + url.PathEscape(version)
	if err := call(ctx, http.MethodGet, path, lang, nil, &r); err != nil {
		return nil, err
	}
	if r.Kind != kind || r.ID != id || (version != LatestVersion && r.Version != version) {
		return nil, &Error{Message: "the Hub returned another release"}
	}
	return &r, nil
}

// Package is a downloaded release whose signature has been verified against its source.
type Package struct {
	Release *Release
	Source  string
	// Signature is the compact JSON of the envelope, to store with the installed package.
	Signature string
	Statement *Statement
}

// Download downloads the source of the release and verifies it; sources larger than maxSize bytes are refused.
func Download(ctx context.Context, kind, id, version, lang string, maxSize int) (*Package, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	r, err := GetRelease(ctx, kind, id, version, lang)
	if err != nil {
		return nil, err
	}
	// The source is only fetched from the Hub, whatever the response says.
	if !strings.HasPrefix(r.SourceURL, baseURL+"/open/v1/packages/") {
		return nil, &Error{Message: "the source is not served by the Hub"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.SourceURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, requestError(err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, &Error{Message: fmt.Sprintf("the server responded %d", resp.StatusCode)}
	}
	source, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxSize)+1))
	if err != nil {
		return nil, requestError(err)
	}
	if len(source) > maxSize {
		return nil, &Error{Message: "the source is too large"}
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, r.Signature); err != nil {
		return nil, &VerifyError{Err: ErrNoSignature}
	}
	st, err := VerifyRelease(compact.String(), Expect{Kind: kind, ID: id, Version: r.Version, Language: r.Language}, string(source))
	if err != nil {
		return nil, &VerifyError{Err: err}
	}
	return &Package{Release: r, Source: string(source), Signature: compact.String(), Statement: st}, nil
}

// Installed is a package installed from the Hub.
type Installed struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

// ReleaseBrief is a version of a package.
type ReleaseBrief struct {
	Version       string     `json:"version"`
	MinAppVersion string     `json:"minAppVersion"`
	Prerelease    bool       `json:"prerelease"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

// Update is the state of an installed package on the Hub.
type Update struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version string `json:"version"`
	// Found is false if the package has never been published on the Hub.
	Found        bool   `json:"found"`
	Yanked       bool   `json:"yanked"`
	YankReason   string `json:"yankReason"`
	Delisted     bool   `json:"delisted"`
	DelistReason string `json:"delistReason"`
	// Latest is the latest release if it is newer than the installed version and compatible, nil otherwise.
	Latest *ReleaseBrief `json:"latest"`
}

// Updates is the result of an update check.
type Updates struct {
	Packages  []*Update
	CheckedAt time.Time
}

var updatesCache struct {
	sync.Mutex
	key    string
	result *Updates
}

// CheckUpdates returns the state of the installed packages on the Hub, reusing the result of the same check for an
// hour unless refresh is true. appVersion is the Sayrud version, the releases requiring a later one are not offered.
func CheckUpdates(ctx context.Context, appVersion string, installed []Installed, refresh bool) (*Updates, error) {
	installed = slices.Clone(installed)
	slices.SortFunc(installed, func(a, b Installed) int {
		return strings.Compare(a.Kind+"/"+a.ID+"@"+a.Version, b.Kind+"/"+b.ID+"@"+b.Version)
	})
	installed = slices.Compact(installed)
	key := appVersion
	for _, p := range installed {
		key += "\n" + p.Kind + "/" + p.ID + "@" + p.Version
	}

	updatesCache.Lock()
	defer updatesCache.Unlock()
	if !refresh && updatesCache.result != nil && updatesCache.key == key && time.Since(updatesCache.result.CheckedAt) < updatesTTL {
		return updatesCache.result, nil
	}

	result := &Updates{Packages: []*Update{}, CheckedAt: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	for chunk := range slices.Chunk(installed, maxUpdatePackages) {
		var resp struct {
			Packages []*Update `json:"packages"`
		}
		body := map[string]interface{}{"appVersion": appVersion, "packages": chunk}
		if err := call(ctx, http.MethodPost, "/open/v1/updates", "", body, &resp); err != nil {
			return nil, err
		}
		result.Packages = append(result.Packages, resp.Packages...)
	}
	updatesCache.key, updatesCache.result = key, result
	return result, nil
}
