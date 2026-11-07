package routeutil

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// memberSeparator splits the members of a cell, the same as the multiple select separator of the frontend.
var memberSeparator = regexp.MustCompile(`[,，、;；\n]`)

// Directory resolves the users of the member and writer values of a project, it is used within a single request or transaction.
// A nil Directory resolves nothing.
type Directory struct {
	ctx       context.Context
	tx        *gorm.DB
	projectID int64

	// members are the owner and the collaborators in order, the candidates when matching emails and names.
	members []*db.User
	// current are the IDs of the members.
	current map[int64]bool
	// users are the loaded users keyed by ID, nil for the deleted ones.
	users map[int64]*db.User
	// workflows are the loaded workflow names keyed by ID.
	workflows map[int64]string
}

// LoadDirectory loads the members of the project, the other users and workflows are loaded when first used.
func LoadDirectory(ctx context.Context, tx *gorm.DB, projectID int64) (*Directory, error) {
	ids, err := db.NewProjectMembersStore(tx).UserIDs(ctx, projectID)
	if err != nil {
		return nil, errors.Wrap(err, "list members")
	}
	d := &Directory{ctx: ctx, tx: tx, projectID: projectID, current: map[int64]bool{}, users: map[int64]*db.User{}, workflows: map[int64]string{}}
	if err := d.Preload(ids, nil); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if user := d.users[id]; user != nil {
			d.members = append(d.members, user)
			d.current[id] = true
		}
	}
	return d, nil
}

// NewDirectory returns the directory of the loaded users and workflows, the members are the users of memberIDs in order; the others are never loaded.
func NewDirectory(users []*db.User, memberIDs []int64, workflows map[int64]string) *Directory {
	d := &Directory{current: map[int64]bool{}, users: lo.KeyBy(users, func(u *db.User) int64 { return u.ID }), workflows: map[int64]string{}}
	for id, name := range workflows {
		d.workflows[id] = name
	}
	for _, id := range memberIDs {
		if user := d.users[id]; user != nil {
			d.members = append(d.members, user)
			d.current[id] = true
		}
	}
	return d
}

// Preload loads the users and the workflows not loaded yet in batch.
func (d *Directory) Preload(userIDs, workflowIDs []int64) error {
	if d == nil || d.tx == nil {
		return nil
	}
	missingUsers := lo.Uniq(lo.Filter(userIDs, func(id int64, _ int) bool { _, ok := d.users[id]; return !ok && id > 0 }))
	if len(missingUsers) > 0 {
		users, err := db.NewUsersStore(d.tx).ListByIDs(d.ctx, missingUsers)
		if err != nil {
			return errors.Wrap(err, "list users")
		}
		for _, id := range missingUsers {
			d.users[id] = nil
		}
		for _, user := range users {
			d.users[user.ID] = user
		}
	}
	missingWorkflows := lo.Uniq(lo.Filter(workflowIDs, func(id int64, _ int) bool { _, ok := d.workflows[id]; return !ok && id > 0 }))
	if len(missingWorkflows) > 0 {
		names, err := db.NewWorkflowsStore(d.tx).NamesByIDs(d.ctx, d.projectID, missingWorkflows)
		if err != nil {
			return errors.Wrap(err, "list workflows")
		}
		for _, id := range missingWorkflows {
			d.workflows[id] = names[id]
		}
	}
	return nil
}

// User returns the user with the ID, or nil if it has been deleted. A lookup failure is treated as deleted, it only affects the names.
func (d *Directory) User(id int64) *db.User {
	if d == nil {
		return nil
	}
	if _, ok := d.users[id]; !ok {
		_ = d.Preload([]int64{id}, nil)
	}
	return d.users[id]
}

// IsMember reports whether the user is the owner or a collaborator of the project.
func (d *Directory) IsMember(id int64) bool {
	return d != nil && d.current[id]
}

// WorkflowName returns the name of the workflow of the project, empty if it does not exist.
func (d *Directory) WorkflowName(id int64) string {
	if d == nil {
		return ""
	}
	if _, ok := d.workflows[id]; !ok {
		_ = d.Preload(nil, []int64{id})
	}
	return d.workflows[id]
}

// UserName returns the name of the user, empty if it has been deleted.
func (d *Directory) UserName(id int64) string {
	if user := d.User(id); user != nil {
		return user.UserName
	}
	return ""
}

// WriterName returns the name of the user or the workflow writing the record, empty if unknown.
func (d *Directory) WriterName(writer db.SLRecordWriter) string {
	if writer.WorkflowID != 0 {
		return d.WorkflowName(writer.WorkflowID)
	}
	if writer.UserID != 0 {
		return d.UserName(writer.UserID)
	}
	return ""
}

// MemberNames returns the names of the users of the member value joined by ", ", the deleted users are skipped.
func (d *Directory) MemberNames(value interface{}) string {
	ids := MemberIDs(value)
	_ = d.Preload(ids, nil)
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name := d.UserName(id); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// Match returns the member whose email equals the text case-insensitively, or else whose name equals the text.
func (d *Directory) Match(text string) (int64, bool) {
	t := strings.TrimSpace(text)
	if d == nil || t == "" {
		return 0, false
	}
	for _, user := range d.members {
		if strings.EqualFold(user.Email, t) {
			return user.ID, true
		}
	}
	for _, user := range d.members {
		if user.UserName == t {
			return user.ID, true
		}
	}
	return 0, false
}

// ParseMembers parses the text of a cell as the member value: the parts split by the separators are matched by Match, the unmatched ones are dropped,
// and only the first member is kept if multiple is false. It returns nil if nothing matches.
func (d *Directory) ParseMembers(text string, multiple bool) []interface{} {
	var ids []interface{}
	for _, part := range memberSeparator.Split(text, -1) {
		id, ok := d.Match(part)
		if !ok || lo.Contains(ids, interface{}(float64(id))) {
			continue
		}
		ids = append(ids, float64(id))
		if !multiple {
			break
		}
	}
	return ids
}

// ResolveMember returns the member identified by an ID (a number or a numeric string) or an email, ok is false if it is not a member of the project.
func (d *Directory) ResolveMember(v interface{}) (int64, bool) {
	if d == nil {
		return 0, false
	}
	var id int64
	switch t := v.(type) {
	case float64:
		id, _ = memberID(t)
	case string:
		s := strings.TrimSpace(t)
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			id = n
		} else if strings.Contains(s, "@") {
			for _, user := range d.members {
				if strings.EqualFold(user.Email, s) {
					id = user.ID
				}
			}
		}
	}
	return id, id > 0 && d.IsMember(id)
}
