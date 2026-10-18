package workflow

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

// ErrInvalidPluginURL means the source URL is not an http or https URL.
var ErrInvalidPluginURL = errors.New("invalid plugin URL")

// DownloadError is a failed download of a plugin source; Message can be shown to the admin.
type DownloadError struct {
	Message string
}

func (e *DownloadError) Error() string {
	return e.Message
}

const pluginDownloadTimeout = 15 * time.Second

// DownloadPluginSource downloads a plugin source with a client that can not reach private networks; the language is TypeScript when the path ends with .ts or .mts.
func DownloadPluginSource(ctx context.Context, rawURL string) (string, sandbox.Language, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return "", "", ErrInvalidPluginURL
	}
	ctx, cancel := context.WithTimeout(ctx, pluginDownloadTimeout)
	defer cancel()
	resp, err := (&DefaultActions{}).PluginFetch(ctx, sandbox.FetchRequest{Method: http.MethodGet, URL: u.String(), Headers: map[string]string{"Accept": "text/javascript, application/javascript, text/plain, */*"}}, []string{"*"})
	if err != nil {
		var actionErr *ActionError
		if errors.As(err, &actionErr) {
			return "", "", &DownloadError{Message: strings.TrimPrefix(actionErr.Message, "fetch: ")}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", "", &DownloadError{Message: "the request timed out"}
		}
		return "", "", err
	}
	if resp.Status != http.StatusOK {
		return "", "", &DownloadError{Message: fmt.Sprintf("the server responded %d", resp.Status)}
	}
	if len(resp.Body) > sandbox.MaxSourceSize {
		return "", "", &DownloadError{Message: "the source is too large"}
	}
	if !utf8.ValidString(resp.Body) {
		return "", "", &DownloadError{Message: "the source is not UTF-8 text"}
	}
	if mediaType, _, _ := mime.ParseMediaType(resp.Headers["content-type"]); strings.HasPrefix(mediaType, "text/html") {
		return "", "", &DownloadError{Message: "the URL returns an HTML page instead of the source"}
	}
	return resp.Body, LanguageOf(u.Path), nil
}

// LanguageOf returns the language of a plugin file by its extension.
func LanguageOf(name string) sandbox.Language {
	switch strings.ToLower(path.Ext(name)) {
	case ".ts", ".mts":
		return sandbox.TypeScript
	}
	return sandbox.JavaScript
}

// InspectPlugin compiles a plugin source and validates its manifest; errors are *sandbox.Error, *ManifestError or the sandbox errors.
func InspectPlugin(ctx context.Context, source string, lang sandbox.Language) (*sandbox.Program, *PluginManifest, error) {
	program, err := sandbox.Compile(ctx, "plugin"+pluginExtension(lang), source, lang)
	if err != nil {
		return nil, nil, err
	}
	m, err := ParsePluginManifest(program.Manifest())
	if err != nil {
		return nil, nil, err
	}
	return program, m, nil
}

// TestPlugin runs execute with the config as is; ctx.fetch sends real requests to the hosts of an action plugin.
func TestPlugin(ctx context.Context, program *sandbox.Program, m *PluginManifest, config map[string]interface{}) (*sandbox.Result, error) {
	if config == nil {
		config = map[string]interface{}{}
	}
	call := sandbox.Call{
		Hook: sandbox.HookExecute,
		Args: []interface{}{config},
		Context: map[string]interface{}{
			"run":     map[string]interface{}{"id": "test", "time": time.Now().UTC().Format(time.RFC3339), "trigger": string(db.WorkflowRunTriggerDebug), "timezone": "UTC"},
			"step":    map[string]interface{}{"id": m.IDBase, "name": ""},
			"project": map[string]interface{}{"uid": "prjtest", "name": "Test"},
			"plugin":  map[string]interface{}{"id": m.ID, "version": m.Version},
			"preview": false,
		},
		Timeout: time.Duration(m.Timeout) * time.Second,
	}
	if m.Action {
		call.Fetch = pluginFetch(&DefaultActions{}, m.Permissions.Fetch)
	}
	result, err := program.Run(ctx, call)
	if err != nil {
		return result, pluginError(err, result.Fetches > 0, m.Timeout)
	}
	return result, nil
}
