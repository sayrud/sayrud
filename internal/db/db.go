package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// Init initializes the database.
func Init() (*gorm.DB, error) {
	dsn := conf.Postgres.DSN

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
		NowFunc: func() time.Time {
			return dbutil.Now()
		},
		Logger: logger.New(
			logrus.New(),
			logger.Config{
				SlowThreshold:             3 * time.Second,
				LogLevel:                  logger.Silent,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return nil, errors.Wrap(err, "open connection")
	}

	tables := []interface{}{
		&User{}, &UserSession{}, &Setting{}, &AuthProvider{}, &UserIdentity{},
		&Project{}, &ProjectMember{},

		&SLTable{}, &SLField{}, &SLRecord{}, &SLView{}, &SLChangeset{},
		&CustomFieldShortcut{}, &SLShortcutJob{}, &SLImport{},

		&File{},

		&Workflow{}, &WorkflowRevision{}, &WorkflowRun{}, &WorkflowJob{}, &WorkflowKey{}, &Credential{},
	}

	// Serialize startup DDL across replicas. A transaction releases the advisory
	// lock even when migration fails or the process loses its connection.
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(0x736179727564)).Error; err != nil {
			return err
		}
		if err := dropLegacyApis(tx); err != nil {
			return errors.Wrap(err, "drop legacy apis")
		}
		if err := dropTableAppearance(tx); err != nil {
			return errors.Wrap(err, "drop table appearance")
		}

		return tx.AutoMigrate(tables...)
	}); err != nil {
		return nil, errors.Wrap(err, "auto migrate")
	}

	SetDatabaseStore(db)

	if err := Users.EnsureAdmin(context.Background()); err != nil {
		return nil, errors.Wrap(err, "ensure admin")
	}

	if err := db.Use(tracing.NewPlugin(
		tracing.WithDBName("sayrud"),
	)); err != nil {
		return nil, errors.Wrap(err, "register otelgorm plugin")
	}

	return db, nil
}

// SetDatabaseStore sets the database table store.
func SetDatabaseStore(db *gorm.DB) {
	Users = NewUsersStore(db)
	UserSessions = NewUserSessionsStore(db)
	Settings = NewSettingsStore(db)
	AuthProviders = NewAuthProvidersStore(db)
	UserIdentities = NewUserIdentitiesStore(db)
	Projects = NewProjectsStore(db)
	ProjectMembers = NewProjectMembersStore(db)

	SLTables = NewSLTablesStore(db)
	SLFields = NewSLFieldsStore(db)
	SLRecords = NewSLRecordsStore(db)
	SLViews = NewSLViewsStore(db)
	SLChangesets = NewSLChangesetsStore(db)
	FieldShortcuts = NewFieldShortcutsStore(db)
	SLShortcutJobs = NewSLShortcutJobsStore(db)
	SLImports = NewSLImportsStore(db)

	Files = NewFilesStore(db)

	Workflows = NewWorkflowsStore(db)
	WorkflowRevisions = NewWorkflowRevisionsStore(db)
	WorkflowRuns = NewWorkflowRunsStore(db)
	WorkflowJobs = NewWorkflowJobsStore(db)
	WorkflowKeys = NewWorkflowKeysStore(db)
	Credentials = NewCredentialsStore(db)
}

// dropLegacyApis drops the legacy apis table designed by kind: it was never wired to the routes, and its unique index limits each project to a single API.
func dropLegacyApis(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("apis") || !tx.Migrator().HasColumn("apis", "kind") {
		return nil
	}
	return tx.Migrator().DropTable("apis")
}

// dropTableAppearance drops the deprecated icon and color columns of the tables.
func dropTableAppearance(tx *gorm.DB) error {
	m := tx.Migrator()
	for _, column := range []string{"icon", "color"} {
		if m.HasColumn("sl_tables", column) {
			if err := m.DropColumn("sl_tables", column); err != nil {
				return errors.Wrapf(err, "drop sl_tables.%s", column)
			}
		}
	}
	return nil
}
