package route

import (
	"net/http"

	"github.com/MEDIGO/go-healthz"
	"github.com/flamego/flamego"

	"github.com/wuhan005/sayrud/frontend"
	appcontext "github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/route/api"
	"github.com/wuhan005/sayrud/internal/setup"
)

// SetupPath is the page of the installation wizard.
const SetupPath = "/setup"

// NewSetup creates the router of the installation wizard, which serves the requests until the application takes over.
func NewSetup(wizard *setup.Wizard) *flamego.Flame {
	f := flamego.Classic()
	frontendHandler := frontend.Handler()

	f.Use(
		staticAssets(frontendHandler),
		i18n.Middleware(),
		appcontext.Contexter(nil),
	)
	f.Map(wizard)

	f.Group("/_", func() {
		f.Group("/setup", func() {
			f.Get("/status", api.Setup.Status)
			f.Group("", func() {
				f.Post("/verify", api.Setup.Verify)
				f.Post("/database/test", form.Bind(form.SetupDatabase{}), api.Setup.TestDatabase)
				f.Post("/database", form.Bind(form.SetupDatabase{}), api.Setup.SaveDatabase)
				f.Post("/admin", form.Bind(form.SetupAdmin{}), api.Setup.CreateAdmin)
			}, api.Setup.Authorizer, api.Setup.LimitBody)
		})
		f.Any("/{path: **}", api.Setup.Unavailable)
	})
	f.Any("/api/{path: **}", api.Setup.Unavailable)

	f.Group("/-", func() {
		f.Get("/healthz", healthz.Handler().ServeHTTP)
		// Ready lets the orchestrators route the installer to the wizard.
		f.Get("/readyz", func(w http.ResponseWriter) { w.WriteHeader(http.StatusOK) })
	})

	f.NotFound(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && (path == "/" || (path != SetupPath && frontend.IsAppRoute(path))) {
			http.Redirect(w, r, SetupPath, http.StatusFound)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})
	return f
}
