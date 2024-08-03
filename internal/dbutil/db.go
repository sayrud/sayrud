package dbutil

import (
	"database/sql"
	"strings"

	"gorm.io/gorm"
)

type Transactor interface {
	Transaction(fc func(tx *gorm.DB) error, opts ...*sql.TxOptions) (err error)
}

func IsUniqueViolation(err error, constraint string) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "duplicate key value violates unique constraint") && strings.Contains(errMsg, constraint)
}
