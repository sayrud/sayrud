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
		&User{}, &UserSession{}, &Setting{},
		&Project{}, &ProjectMember{},

		&SLTable{}, &SLField{}, &SLRecord{}, &SLView{}, &SLChangeset{},

		&Api{},
	}
	if err := db.AutoMigrate(tables...); err != nil {
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
	Projects = NewProjectsStore(db)
	ProjectMembers = NewProjectMembersStore(db)

	SLTables = NewSLTablesStore(db)
	SLFields = NewSLFieldsStore(db)
	SLRecords = NewSLRecordsStore(db)
	SLViews = NewSLViewsStore(db)
	SLChangesets = NewSLChangesetsStore(db)

	Apis = NewApisStore(db)
}
