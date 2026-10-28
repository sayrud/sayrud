package db

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NavigationPosition is the current sidebar position of a table or workflow.
type NavigationPosition struct {
	UID      string
	Position int
}

type navigationItem struct {
	NavigationPosition
	ID   int64
	Kind int
}

var ErrNavigationItemNotFound = errors.New("navigation item does not exist in the project")

func lockProjectNavigation(ctx context.Context, tx *gorm.DB, projectID int64) error {
	var project Project
	err := tx.WithContext(ctx).Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Take(&project, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrProjectNotFound
	}

	return err
}

func nextProjectNavigationPosition(ctx context.Context, tx *gorm.DB, projectID int64) (int, error) {
	if err := lockProjectNavigation(ctx, tx, projectID); err != nil {
		return 0, err
	}

	var position int
	err := tx.WithContext(ctx).Raw(`SELECT GREATEST(
		COALESCE((SELECT MAX(position) FROM sl_tables WHERE project_id = ? AND deleted_at IS NULL), -1),
		COALESCE((SELECT MAX(position) FROM workflows WHERE project_id = ? AND deleted_at IS NULL), -1)
	) + 1`, projectID, projectID).Scan(&position).Error

	return position, err
}

func listProjectNavigation(ctx context.Context, tx *gorm.DB, projectID int64) ([]navigationItem, error) {
	items := []navigationItem{}
	err := tx.WithContext(ctx).Raw(`SELECT id, uid, position, kind FROM (
		SELECT id, uid, position, 0 AS kind, id AS stable_id FROM sl_tables WHERE project_id = ? AND deleted_at IS NULL
		UNION ALL SELECT id, uid, position, 1 AS kind, -id AS stable_id FROM workflows WHERE project_id = ? AND deleted_at IS NULL
	) AS items ORDER BY position ASC, kind ASC, stable_id ASC`, projectID, projectID).Scan(&items).Error

	return items, err
}

// writeNavigationPositions uses at most two updates, without changing content timestamps.
func writeNavigationPositions(ctx context.Context, tx *gorm.DB, items []navigationItem) error {
	for kind, table := range []string{"sl_tables", "workflows"} {
		var values []string
		var args []interface{}

		for _, item := range items {
			if item.Kind != kind {
				continue
			}

			values = append(values, "(?::bigint, ?::bigint)")
			args = append(args, item.ID, item.Position)
		}

		if len(values) == 0 {
			continue
		}

		query := fmt.Sprintf(`UPDATE %s AS item SET position = positions.position
			FROM (VALUES %s) AS positions(id, position) WHERE item.id = positions.id`, table, strings.Join(values, ","))
		if err := tx.WithContext(ctx).Exec(query, args...).Error; err != nil {
			return errors.Wrap(err, "update navigation positions")
		}
	}

	return nil
}

func (db *projects) MoveNavigation(ctx context.Context, projectID int64, uid, targetUID string, after bool) ([]NavigationPosition, error) {
	var result []NavigationPosition
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProjectNavigation(ctx, tx, projectID); err != nil {
			return err
		}

		items, err := listProjectNavigation(ctx, tx, projectID)
		if err != nil {
			return err
		}

		from := slices.IndexFunc(items, func(item navigationItem) bool { return item.UID == uid })
		to := slices.IndexFunc(items, func(item navigationItem) bool { return item.UID == targetUID })
		if from < 0 || to < 0 {
			return ErrNavigationItemNotFound
		}

		if from != to {
			moved := items[from]
			positions := make([]int, len(items))
			for i, item := range items {
				positions[i] = item.Position
			}

			items = slices.Delete(items, from, from+1)
			if from < to {
				to--
			}
			if after {
				to++
			}
			items = slices.Insert(items, to, moved)

			changed := make([]navigationItem, 0, max(from, to)-min(from, to)+1)
			for i := min(from, to); i <= max(from, to); i++ {
				items[i].Position = positions[i]
				changed = append(changed, items[i])
			}

			if err := writeNavigationPositions(ctx, tx, changed); err != nil {
				return err
			}
		}

		result = make([]NavigationPosition, len(items))
		for i, item := range items {
			result[i] = item.NavigationPosition
		}

		return nil
	})

	return result, err
}
