package collab

import (
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// ShareSession binds a read-only connection to the share and password grant used at the handshake.
type ShareSession struct {
	TableID      int64
	Token        string
	PasswordHash string
	ExpiresAt    time.Time
}

func (s *ShareSession) root(ctx context.Context, projectID int64) (*db.SLTable, error) {
	if !s.ExpiresAt.IsZero() && !time.Now().Before(s.ExpiresAt) {
		return nil, db.ErrSLTableNotFound
	}

	table, err := db.SLTables.GetByID(ctx, s.TableID)
	if err != nil {
		return nil, err
	}
	if table.ProjectID != projectID || !table.ShareEnabled || table.ShareToken != s.Token || table.SharePasswordHash != s.PasswordHash {
		return nil, db.ErrSLTableNotFound
	}

	if _, err := db.Projects.GetByID(ctx, projectID); err != nil {
		return nil, err
	}

	return table, nil
}

func sharedTableAllowed(ctx context.Context, root *db.SLTable, tableUID string) bool {
	if tableUID == root.UID {
		return true
	}
	if !root.ShareIncludeChildren || tableUID == "" {
		return false
	}

	table, err := db.SLTables.GetByUID(ctx, tableUID)
	return err == nil && table.ProjectID == root.ProjectID
}

// PrepareSharedMetadata removes private field configuration from a public response.
func PrepareSharedMetadata(metadata map[string]interface{}) map[string]interface{} {
	metadata = maps.Clone(metadata)
	delete(metadata, "optionsReference")
	return metadata
}

// PrepareSharedValues rewrites attachment descriptors, the only stored cell values containing objects.
func PrepareSharedValues(values map[string]interface{}, token, tableUID string) {
	for _, value := range values {
		files, _ := value.([]interface{})
		for _, value := range files {
			file, ok := value.(map[string]interface{})
			uid, _ := file["uid"].(string)
			if ok && uid != "" {
				file["url"] = "/_/shares/" + url.PathEscape(token) + "/tables/" + url.PathEscape(tableUID) + "/attachments/" + url.PathEscape(uid)
			}
		}
	}
}

// PrepareSharedChangeset applies the same filtering to live messages and revision catch-up responses.
// Its input is independently decoded for each recipient, so member messages remain unchanged.
func PrepareSharedChangeset(changeset *Changeset, token string) {
	operations := make([]Operation, 0, len(changeset.Operations))
	for _, operation := range changeset.Operations {
		actions := make([]Action, 0, len(operation.Actions))
		for _, action := range operation.Actions {
			if action.Action == ActionSetFieldShortcut {
				continue
			}

			action.Shortcut = nil
			if action.Field != nil {
				action.Field.Shortcut = nil
				action.Field.Metadata = PrepareSharedMetadata(action.Field.Metadata)
			}
			PrepareSharedValues(action.Values, token, changeset.TableUID)
			actions = append(actions, action)
		}
		if len(actions) > 0 {
			operations = append(operations, Operation{Actions: actions})
		}
	}
	changeset.Operations = operations
}

// sharedMessage rechecks access immediately before delivery and filters private configuration.
func (c *Client) sharedMessage(ctx context.Context, raw []byte) ([]byte, bool) {
	root, err := c.share.root(ctx, c.project.ID)
	if err != nil {
		return nil, false
	}

	var message Message
	if json.Unmarshal(raw, &message) != nil {
		return nil, true
	}

	switch message.Type {
	case MessageNewChanges:
		var changeset Changeset
		if json.Unmarshal(message.Data, &changeset) != nil || !sharedTableAllowed(ctx, root, changeset.TableUID) {
			return nil, true
		}

		PrepareSharedChangeset(&changeset, c.share.Token)
		return newMessage(MessageNewChanges, 0, changeset), true

	case MessageMembers:
		var data membersData
		if json.Unmarshal(message.Data, &data) != nil {
			return nil, true
		}

		members := make([]Member, 0, len(data.Members))
		for _, member := range data.Members {
			if member.ClientID == c.member.ClientID || sharedTableAllowed(ctx, root, member.TableUID) {
				members = append(members, member)
			}
		}
		return newMessage(MessageMembers, 0, membersData{Members: members}), true

	case MessageShortcutJobs:
		return nil, true
	}

	return raw, true
}

// NotifyShareChanged makes existing visitors reload the scope or verify the new password.
func (h *Hub) NotifyShareChanged(projectUID string, tableID int64) {
	for _, c := range h.clients(projectUID) {
		if c.share != nil && c.share.TableID == tableID {
			c.send(newMessage(MessageShareChanged, 0, nil))
			c.close()
		}
	}
}
