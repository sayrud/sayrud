package db

import (
	"github.com/cockroachdb/errors"
	"gorm.io/gorm"
)

// initializeProjectNavigationPositions preserves the default order when adding position columns.
func initializeProjectNavigationPositions(tx *gorm.DB) error {
	m := tx.Migrator()
	if m.HasColumn(&SLTable{}, "Position") && m.HasColumn(&Workflow{}, "Position") {
		return nil
	}

	// Block navigation changes until existing records have their positions assigned.
	if m.HasTable("projects") {
		if err := tx.Exec("LOCK TABLE projects IN ACCESS EXCLUSIVE MODE").Error; err != nil {
			return err
		}
	}

	if err := tx.AutoMigrate(&SLTable{}, &Workflow{}); err != nil {
		return errors.Wrap(err, "add navigation positions")
	}

	var projectIDs []int64
	if err := tx.Raw(`SELECT project_id FROM sl_tables WHERE deleted_at IS NULL
		UNION SELECT project_id FROM workflows WHERE deleted_at IS NULL`).Scan(&projectIDs).Error; err != nil {
		return errors.Wrap(err, "list navigation projects")
	}

	ctx := tx.Statement.Context
	for _, projectID := range projectIDs {
		items, err := listProjectNavigation(ctx, tx, projectID)
		if err != nil {
			return err
		}

		for i := range items {
			items[i].Position = i
		}

		if err := writeNavigationPositions(ctx, tx, items); err != nil {
			return err
		}
	}

	return nil
}
