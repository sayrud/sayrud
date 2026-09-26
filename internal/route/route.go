package route

import (
	"github.com/flamego/flamego"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/route/api"
	"github.com/wuhan005/sayrud/internal/tracing"
)

// New creates the router of the application.
// @Title Sayrud API
// @Version 1.0
// @BasePath /_
func New(db *gorm.DB, redisClient *redis.Client) *flamego.Flame {
	f := flamego.Classic()

	f.Use(
		tracing.Middleware("sayrud"),
		context.Contexter(db, redisClient),
	)

	f.Group("/_", func() {
		f.Group("/auth", func() {
			f.Get("/profile", api.Auth.Profile)
		}, api.Auth.Authenticator)

		f.Group("/projects", func() {
			f.Combo("").
				Get(api.Project.ListProjects).
				Post(form.Bind(form.CreateProject{}), api.Project.CreateProject)

			f.Group("/{projectUID}", func() {
				f.Combo("").
					Get(api.Project.GetProject).
					Put(form.Bind(form.UpdateProject{}), api.Project.UpdateProject).
					Delete(api.Project.DeleteProject)

				f.Group("/tables", func() {
					f.Combo("").
						Get(api.Schemaless.ListTables).
						Post(form.Bind(form.CreateTable{}), api.Schemaless.CreateTable)
					f.Get("/types", api.Schemaless.FieldTypes)

					f.Group("/{tableUID}", func() {
						f.Combo("").
							Get(api.Schemaless.GetTable).
							Put(form.Bind(form.UpdateTable{}), api.Schemaless.UpdateTable).
							Delete(api.Schemaless.DeleteTable)

						f.Group("/fields", func() {
							f.Combo("").
								Get(api.Schemaless.ListFields).
								Post(form.Bind(form.CreateFields{}), api.Schemaless.CreateFields)

							f.Group("/{fieldUID}", func() {
								f.Combo("").
									Put(form.Bind(form.UpdateField{}), api.Schemaless.UpdateField).
									Delete(api.Schemaless.DeleteField)
								f.Put("/position", form.Bind(form.UpdateFieldPosition{}), api.Schemaless.UpdateFieldPosition)
							}, api.Schemaless.Fielder)
						})

						// Record API routes.
						f.Group("/records", func() {
							f.Combo("").
								Get(api.Schemaless.ListRecords).
								Post(form.Bind(form.CreateRecord{}), api.Schemaless.CreateRecord)
							f.Post("/batch", form.Bind(form.BatchCreateRecords{}), api.Schemaless.BatchCreateRecords)
							f.Post("/query", form.Bind(form.QueryRecords{}), api.Schemaless.QueryRecords)
							f.Group("/{recordUID}", func() {
								f.Combo("").
									Get(api.Schemaless.GetRecord).
									Put(form.Bind(form.UpdateRecord{}), api.Schemaless.UpdateRecord).
									Delete(api.Schemaless.DeleteRecord)
							}, api.Schemaless.Recorder)
						})
					}, api.Schemaless.Tabler)
				})

				f.Group("/ai", func() {
					f.Post("/advice", form.Bind(form.AIAdvice{}), api.AI.Advice)
					f.Post("/apply", form.Bind(form.AIApply{}), api.AI.Apply)
				})
			}, api.Project.Projecter)
		}, api.Auth.Authenticator)
	})

	f.Get("/healthz")

	return f
}
