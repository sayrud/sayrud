package route

import (
	"net/http"

	"github.com/MEDIGO/go-healthz"
	"github.com/flamego/flamego"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/frontend"
	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/observability/tracing"
	"github.com/wuhan005/sayrud/internal/route/api"
)

// Options contains the dependencies used by the application router.
type Options struct {
	DB             *gorm.DB
	MetricsHandler http.Handler
}

// New creates the router of the application.
// @Title Sayrud API
// @Version 1.0
// @BasePath /_
func New(opts Options) *flamego.Flame {
	f := flamego.Classic()

	f.Use(
		tracing.Middleware("sayrud"),
		context.Contexter(opts.DB),
	)
	f.Map(collab.NewHub(opts.DB))

	canEdit := api.Project.RequireRole(db.ProjectRoleEditor)
	canManage := api.Project.RequireRole(db.ProjectRoleManager)
	isOwner := api.Project.RequireRole(db.ProjectRoleOwner)

	f.Group("/_", func() {
		f.Group("/auth", func() {
			f.Post("/sign-up", form.Bind(form.SignUp{}), api.Auth.SignUp)
			f.Post("/sign-in", form.Bind(form.SignIn{}), api.Auth.SignIn)
			f.Post("/sign-out", api.Auth.SignOut)

			f.Combo("/profile", api.Auth.Authenticator).
				Get(api.Auth.Profile).
				Put(form.Bind(form.UpdateProfile{}), api.Auth.UpdateProfile)
			f.Put("/password", api.Auth.Authenticator, form.Bind(form.UpdatePassword{}), api.Auth.UpdatePassword)
		})

		f.Group("/projects", func() {
			f.Combo("").
				Get(api.Project.ListProjects).
				Post(form.Bind(form.CreateProject{}), api.Project.CreateProject)

			f.Group("/{projectUID}", func() {
				f.Combo("").
					Get(api.Project.GetProject).
					Put(canEdit, form.Bind(form.UpdateProject{}), api.Project.UpdateProject).
					Delete(isOwner, api.Project.DeleteProject)

				f.Group("/members", func() {
					f.Combo("").
						Get(api.Project.ListMembers).
						Post(canManage, form.Bind(form.AddProjectMember{}), api.Project.AddMember)
					f.Get("/lookup", canManage, api.Project.LookupMemberCandidate)
					f.Combo("/{userID}").
						Put(canManage, form.Bind(form.UpdateProjectMember{}), api.Project.UpdateMember).
						Delete(api.Project.RemoveMember)
				})

				f.Get("/ws", api.Collab.Serve)

				f.Group("/tables", func() {
					f.Combo("").
						Get(api.Schemaless.ListTables).
						Post(canEdit, form.Bind(form.CreateTable{}), api.Schemaless.CreateTable)
					f.Get("/types", api.Schemaless.FieldTypes)

					f.Group("/{tableUID}", func() {
						f.Combo("").
							Get(api.Schemaless.GetTable).
							Put(canEdit, form.Bind(form.UpdateTable{}), api.Schemaless.UpdateTable).
							Delete(canEdit, api.Schemaless.DeleteTable)
						f.Get("/snapshot", api.Collab.GetSnapshot)
						f.Get("/changesets", api.Collab.ListChangesets)
						f.Get("/views", api.Collab.ListViews)

						f.Group("/fields", func() {
							f.Combo("").
								Get(api.Schemaless.ListFields).
								Post(canEdit, form.Bind(form.CreateFields{}), api.Schemaless.CreateFields)

							f.Group("/{fieldUID}", func() {
								f.Combo("").
									Put(canEdit, form.Bind(form.UpdateField{}), api.Schemaless.UpdateField).
									Delete(canEdit, api.Schemaless.DeleteField)
								f.Put("/position", canEdit, form.Bind(form.UpdateFieldPosition{}), api.Schemaless.UpdateFieldPosition)
							}, api.Schemaless.Fielder)
						})

						// Record API routes.
						f.Group("/records", func() {
							f.Combo("").
								Get(api.Schemaless.ListRecords).
								Post(canEdit, form.Bind(form.CreateRecord{}), api.Schemaless.CreateRecord)
							f.Post("/batch", canEdit, form.Bind(form.BatchCreateRecords{}), api.Schemaless.BatchCreateRecords)
							f.Post("/query", form.Bind(form.QueryRecords{}), api.Schemaless.QueryRecords)
							f.Post("/fetch", form.Bind(form.FetchRecords{}), api.Collab.FetchRecords)
							f.Group("/{recordUID}", func() {
								f.Combo("").
									Get(api.Schemaless.GetRecord).
									Put(canEdit, form.Bind(form.UpdateRecord{}), api.Schemaless.UpdateRecord).
									Delete(canEdit, api.Schemaless.DeleteRecord)
							}, api.Schemaless.Recorder)
						})
					}, api.Schemaless.Tabler)
				})

				f.Group("/ai", func() {
					f.Post("/advice", canEdit, form.Bind(form.AIAdvice{}), api.AI.Advice)
					f.Post("/apply", canEdit, form.Bind(form.AIApply{}), api.AI.Apply)
				})
			}, api.Project.Projecter)
		}, api.Auth.Authenticator)
	})

	f.Group("/-", func() {
		f.Get("/healthz", healthz.Handler().ServeHTTP)
		if opts.MetricsHandler != nil {
			f.Get("/metrics", opts.MetricsHandler.ServeHTTP)
		}
	})

	f.NotFound(frontend.Handler().ServeHTTP)
	return f
}
