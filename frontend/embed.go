package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var assets embed.FS

// Handler serves the bundled frontend, including Vue Router's base pages.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}

	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/base/") {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		files.ServeHTTP(w, r)
	})
}
