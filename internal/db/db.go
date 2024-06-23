package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var AllTables = []interface{}{
	&User{},
	&Project{},

	&SLTable{},
	&SLField{},
	&SLRecord{},
	&Api{},
}

var dbInstance *gorm.DB

// Init initializes the database.
func Init() (*gorm.DB, error) {
	dsn := os.Getenv("POSTGRES_DSN")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
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
	dbInstance = db

	// Migrate databases.
	if err := db.AutoMigrate(AllTables...); err != nil {
		return nil, errors.Wrap(err, "auto migrate")
	}

	SetDatabaseStore(db)

	return db, nil
}

// SetDatabaseStore sets the database table store.
func SetDatabaseStore(db *gorm.DB) {
	Users = NewUsersStore(db)
	Projects = NewProjectsStore(db)
	SLTables = NewSLTablesStore(db)
	SLFields = NewSLFieldsStore(db)
	SLRecords = NewSLRecordsStore(db)
	Apis = NewApisStore(db)
}

func TruncateAll(ctx context.Context) error {
	for _, model := range AllTables {
		model := model

		stmt := &gorm.Statement{DB: dbInstance}
		if err := stmt.Parse(model); err != nil {
			return errors.Wrap(err, "parse")
		}
		tableName := stmt.Schema.Table

		if err := dbInstance.WithContext(ctx).Exec(fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE;", tableName)).Error; err != nil {
			return errors.Wrap(err, "truncate")
		}
	}

	return nil
}
