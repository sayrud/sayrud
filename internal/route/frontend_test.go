package route

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestBundledFrontend(t *testing.T) {
	router := New(Options{})

	request := func(method, path string) *httptest.ResponseRecorder {
		t.Helper()

		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, path, nil))

		return response
	}

	index := request(http.MethodGet, "/")
	if index.Code != http.StatusOK || !strings.Contains(index.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("index: status %d, headers %v", index.Code, index.Header())
	}

	for _, path := range []string{"/base/project/table/view", "/login", "/register"} {
		page := request(http.MethodGet, path)
		if page.Code != http.StatusOK || page.Body.String() != index.Body.String() {
			t.Fatalf("direct Vue Router navigation to %s did not serve the index", path)
		}
	}

	asset := regexp.MustCompile(`src="(/assets/[^" ]+\.js)"`).FindStringSubmatch(index.Body.String())
	if len(asset) != 2 {
		t.Fatal("index has no bundled JavaScript asset")
	}

	js := request(http.MethodGet, asset[1])
	if js.Code != http.StatusOK || !strings.Contains(js.Header().Get("Content-Type"), "javascript") || js.Body.Len() == 0 {
		t.Fatalf("JavaScript asset: status %d, headers %v", js.Code, js.Header())
	}

	head := request(http.MethodHead, "/base/project")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatal("HEAD must serve headers without a body")
	}

	for _, path := range []string{"/assets/missing.js", "/_/missing", "/-/metrics"} {
		if response := request(http.MethodGet, path); response.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, response.Code)
		}
	}

	if response := request(http.MethodPost, "/base/project"); response.Code != http.StatusNotFound {
		t.Errorf("POST must not serve the frontend: got %d", response.Code)
	}
}
