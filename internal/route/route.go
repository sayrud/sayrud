package route

import (
	"github.com/flamego/flamego"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/route/api"
)

func New(db *gorm.DB) *flamego.Flame {
	f := flamego.Classic()

	f.Use(context.Contexter(db))

	f.Group("/api", func() {
		f.Group("/projects", func() {
			f.Combo("").
				Get().
				Post()

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
							Post(form.Bind(form.CreateFields{}), api.Schemaless.CreateFields).
							Put(form.Bind(form.UpdateFields{}), api.Schemaless.UpdateFields)

						f.Group("/{fieldUID}", func() {
							f.Combo("").
								Put(form.Bind(form.UpdateField{}), api.Schemaless.UpdateField).
								Delete(api.Schemaless.DeleteField)
						}, api.Schemaless.Fielder)

					})

					// Record API routes.
					f.Group("/records", func() {
						f.Combo("").
							Get(api.Schemaless.ListRecords).
							Post(form.Bind(form.CreateRecord{}), api.Schemaless.CreateRecord)
						f.Group("/{recordUID}", func() {
							f.Combo("").
								Get(api.Schemaless.GetRecord).
								Put(form.Bind(form.UpdateRecord{}), api.Schemaless.UpdateRecord).
								Delete(api.Schemaless.DeleteRecord)
						}, api.Schemaless.Recorder)
					})
				}, api.Schemaless.Tabler)
			})
		})
	})

	f.Get("/healthz")

	return f
}
