package collab

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// refreshAccess rechecks persistent access on inbound messages and heartbeats. Pub/Sub
// notifications accelerate revocation, but delivery is never an auth boundary.
func (c *Client) refreshAccess(ctx context.Context) bool {
	if c.share != nil || c.hub.db == nil {
		return true
	}

	role, err := c.currentRole(ctx, c.hub.db)
	if err != nil {
		c.send(newMessage(MessagePermissionChanged, 0, permissionChangedData{ProjectUID: c.projectUID}))
		c.close()
		return false
	}

	c.mu.Lock()
	changed := c.role != role
	c.role = role
	c.canEdit = role.AtLeast(db.ProjectRoleEditor)
	c.mu.Unlock()

	if changed {
		c.send(newMessage(MessagePermissionChanged, 0, permissionChangedData{ProjectUID: c.projectUID, Role: string(role)}))
	}

	return true
}

// currentRole also runs inside the mutation transaction after waiting for the
// shared table lock, so a queued write cannot use an earlier permission snapshot.
func (c *Client) currentRole(ctx context.Context, gormDB *gorm.DB) (db.ProjectRole, error) {
	if c.sessionToken != "" {
		session, err := db.NewUserSessionsStore(gormDB).GetByToken(ctx, c.sessionToken)
		if err != nil {
			return "", err
		}
		if session.UserID != c.userID {
			return "", fmt.Errorf("session belongs to another user")
		}
	}

	user, err := db.NewUsersStore(gormDB).GetByID(ctx, c.userID)
	if err != nil {
		return "", err
	}
	if user.Disabled() {
		return "", fmt.Errorf("user disabled")
	}

	project, err := db.NewProjectsStore(gormDB).GetByID(ctx, c.project.ID)
	if err != nil {
		return "", err
	}
	if project.OwnerUserID == c.userID {
		return db.ProjectRoleOwner, nil
	}

	return db.NewProjectMembersStore(gormDB).GetRole(ctx, project.ID, c.userID)
}

func (c *Client) revisions(ctx context.Context) (map[string]int64, error) {
	revisions := make(map[string]int64)
	if c.hub.db == nil {
		return revisions, nil
	}

	c.mu.Lock()
	ids := make([]int64, 0, len(c.tables))
	for _, table := range c.tables {
		if table != nil {
			ids = append(ids, table.ID)
		}
	}
	c.mu.Unlock()

	if len(ids) == 0 {
		return revisions, nil
	}

	var tables []db.SLTable
	if err := c.hub.db.WithContext(ctx).Select("uid", "rev", "project_id").Where("id IN ?", ids).Find(&tables).Error; err != nil {
		return nil, err
	}

	var root *db.SLTable
	if c.share != nil {
		var err error
		root, err = c.share.root(ctx, c.project.ID)
		if err != nil {
			return nil, err
		}
	}

	for _, table := range tables {
		if root != nil && table.UID != root.UID && (!root.ShareIncludeChildren || table.ProjectID != root.ProjectID) {
			continue
		}
		revisions[table.UID] = table.Rev
	}

	return revisions, nil
}
