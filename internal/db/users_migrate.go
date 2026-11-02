package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"
)

// assignBotOwners hands the bots without an owner to the enabled admin with the smallest ID, and the projects owned by bots to their owners, since every bot must be owned by a member and bots never own projects.
func assignBotOwners(tx *gorm.DB) error {
	if err := tx.Exec(`UPDATE users SET owner_id = admins.id FROM (
		SELECT MIN(id) AS id FROM users WHERE is_admin AND kind <> ? AND deleted_at IS NULL AND disabled_at IS NULL
	) admins WHERE users.kind = ? AND users.owner_id = 0 AND admins.id IS NOT NULL`, UserKindBot, UserKindBot).Error; err != nil {
		return errors.Wrap(err, "assign owners")
	}

	var rows []struct {
		ID      int64
		OwnerID int64
	}
	if err := tx.Raw(`SELECT projects.id, users.owner_id FROM projects
		JOIN users ON users.id = projects.owner_user_id
		WHERE users.kind = ? AND users.owner_id <> 0 AND projects.deleted_at IS NULL`, UserKindBot).Scan(&rows).Error; err != nil {
		return errors.Wrap(err, "list projects owned by bots")
	}
	for _, r := range rows {
		if err := NewProjectsStore(tx).TransferOwner(context.Background(), r.ID, r.OwnerID); err != nil {
			return errors.Wrapf(err, "transfer project %d", r.ID)
		}
	}
	return nil
}
