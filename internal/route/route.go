package route

import (
	"context"
	"net/http"

	"github.com/MEDIGO/go-healthz"
	"github.com/flamego/flamego"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/frontend"
	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/conf"
	appcontext "github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/observability/tracing"
	"github.com/wuhan005/sayrud/internal/route/api"
	"github.com/wuhan005/sayrud/internal/shortcut"
)

// Options contains the dependencies used by the application router.
type Options struct {
	// DB is the database connection.
	DB *gorm.DB
	// MetricsHandler serves /-/metrics, the route is not registered if it is nil.
	MetricsHandler http.Handler
	// Context runs the background workers of the field shortcuts until it is canceled, they are not started if it is nil.
	Context context.Context
}

// New creates the router of the application.
// @Title Sayrud API
// @Version 1.0
// @BasePath /_
func New(opts Options) *flamego.Flame {
	f := flamego.Classic()

	f.Use(
		tracing.Middleware("sayrud"),
		i18n.Middleware(),
		appcontext.Contexter(opts.DB),
	)
	hub := collab.NewHub(opts.DB)
	engine := shortcut.NewEngine(opts.DB, hub, shortcut.NewExecutor(), conf.Shortcut.Workers)
	if opts.Context != nil {
		engine.Start(opts.Context)
	}
	f.Map(hub, engine)

	canEdit := api.Project.RequireRole(db.ProjectRoleEditor)
	canManage := api.Project.RequireRole(db.ProjectRoleManager)
	isOwner := api.Project.RequireRole(db.ProjectRoleOwner)

	f.Group("/_", func() {
		f.Get("/site", api.Site.Get)

		f.Group("/auth", func() {
			f.Post("/sign-up", form.Bind(form.SignUp{}), api.Auth.SignUp)
			f.Post("/sign-in", form.Bind(form.SignIn{}), api.Auth.SignIn)
			f.Post("/sign-out", api.Auth.SignOut)

			f.Combo("/profile", api.Auth.Authenticator).
				Get(api.Auth.Profile).
				Put(form.Bind(form.UpdateProfile{}), api.Auth.UpdateProfile)
			f.Put("/password", api.Auth.Authenticator, form.Bind(form.UpdatePassword{}), api.Auth.UpdatePassword)
			f.Group("/sessions", func() {
				f.Combo("").
					Get(api.Account.ListSessions).
					Delete(api.Account.RevokeOtherSessions)
				f.Delete("/{sessionID}", api.Account.RevokeSession)
			}, api.Auth.Authenticator)
			f.Combo("/settings", api.Auth.Authenticator).
				Get(api.Account.GetSettings).
				Put(form.Bind(form.UpdateUserSettings{}), api.Account.UpdateSettings)
			f.Delete("/account", api.Auth.Authenticator, form.Bind(form.DeleteAccount{}), api.Account.DeleteAccount)

			f.Group("/sso/{slug}", func() {
				f.Get("/start", api.SSO.Start)
				f.Get("/callback", api.SSO.Callback)
				f.Post("/acs", api.SSO.ACS)
				f.Get("/metadata", api.SSO.Metadata)
			})
			f.Post("/ldap/{slug}/sign-in", form.Bind(form.LDAPSignIn{}), api.SSO.LDAPSignIn)
			f.Group("/identities", func() {
				f.Get("", api.SSO.ListIdentities)
				f.Delete("/{identityID}", api.SSO.DeleteIdentity)
			}, api.Auth.Authenticator)
		})

		f.Group("/admin", func() {
			f.Get("/overview", api.Admin.Overview)
			f.Group("/users", func() {
				f.Combo("").
					Get(api.Admin.ListUsers).
					Post(form.Bind(form.AdminCreateUser{}), api.Admin.CreateUser)
				f.Group("/{userID}", func() {
					f.Combo("").
						Put(form.Bind(form.AdminUpdateUser{}), api.Admin.UpdateUser).
						Delete(api.Admin.DeleteUser)
					f.Put("/admin", form.Bind(form.AdminSetUserAdmin{}), api.Admin.SetUserAdmin)
					f.Put("/status", form.Bind(form.AdminSetUserStatus{}), api.Admin.SetUserStatus)
					f.Put("/password", form.Bind(form.AdminResetPassword{}), api.Admin.ResetUserPassword)
					f.Delete("/sessions", api.Admin.RevokeUserSessions)
				}, api.Admin.Targeter)
			})
			f.Group("/projects", func() {
				f.Get("", api.Admin.ListProjects)
				f.Group("/{projectUID}", func() {
					f.Delete("", api.Admin.DeleteProject)
					f.Put("/owner", form.Bind(form.AdminTransferProject{}), api.Admin.TransferProject)
				}, api.Admin.Projecter)
			})
			f.Combo("/settings").
				Get(api.Admin.GetSettings).
				Put(form.Bind(form.UpdateSystemSettings{}), api.Admin.UpdateSettings)
			f.Group("/auth-providers", func() {
				f.Combo("").
					Get(api.Admin.ListAuthProviders).
					Post(form.Bind(form.CreateAuthProvider{}), api.Admin.CreateAuthProvider)
				f.Put("/positions", form.Bind(form.SetAuthProviderPositions{}), api.Admin.SetAuthProviderPositions)
				f.Post("/test", form.Bind(form.TestAuthProvider{}), api.Admin.TestAuthProvider)
				f.Combo("/{providerID}", api.Admin.AuthProviderer).
					Put(form.Bind(form.UpdateAuthProvider{}), api.Admin.UpdateAuthProvider).
					Delete(api.Admin.DeleteAuthProvider)
			})
			f.Group("/ai", func() {
				f.Combo("").
					Get(api.Admin.GetAISettings).
					Put(form.Bind(form.UpdateAISettings{}), api.Admin.UpdateAISettings)
				f.Post("/test", form.Bind(form.UpdateAISettings{}), api.Admin.TestAISettings)
			})
			f.Group("/field-shortcuts", func() {
				f.Combo("").
					Get(api.Admin.ListFieldShortcuts).
					Post(form.Bind(form.SaveFieldShortcut{}), api.Admin.CreateFieldShortcut)
				f.Post("/test", form.Bind(form.TestFieldShortcut{}), api.Admin.TestFieldShortcut)
				f.Combo("/{shortcutUID}", api.Admin.FieldShortcuter).
					Get(api.Admin.GetFieldShortcut).
					Put(form.Bind(form.SaveFieldShortcut{}), api.Admin.UpdateFieldShortcut).
					Delete(api.Admin.DeleteFieldShortcut)
			})
		}, api.Auth.Authenticator, api.Admin.RequireAdmin)

		f.Group("/projects", func() {
			f.Combo("").
				Get(api.Project.ListProjects).
				Post(form.Bind(form.CreateProject{}), api.Project.CreateProject)

			f.Group("/{projectUID}", func() {
				f.Combo("").
					Get(api.Project.GetProject).
					Put(canEdit, form.Bind(form.UpdateProject{}), api.Project.UpdateProject).
					Delete(isOwner, api.Project.DeleteProject)
				f.Put("/owner", isOwner, form.Bind(form.TransferProjectOwner{}), api.Project.TransferOwner)

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
				f.Get("/field-shortcuts", api.Shortcut.List)

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
						f.Get("/shortcut-jobs", api.Shortcut.ListJobs)
						f.Post("/shortcuts/preview", canEdit, form.Bind(form.PreviewFieldShortcut{}), api.Shortcut.Preview)

						f.Group("/fields", func() {
							f.Combo("").
								Get(api.Schemaless.ListFields).
								Post(canEdit, form.Bind(form.CreateFields{}), api.Schemaless.CreateFields)

							f.Group("/{fieldUID}", func() {
								f.Combo("").
									Put(canEdit, form.Bind(form.UpdateField{}), api.Schemaless.UpdateField).
									Delete(canEdit, api.Schemaless.DeleteField)
								f.Put("/position", canEdit, form.Bind(form.UpdateFieldPosition{}), api.Schemaless.UpdateFieldPosition)
								f.Post("/shortcut/run", canEdit, form.Bind(form.RunFieldShortcut{}), api.Shortcut.Run)
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
