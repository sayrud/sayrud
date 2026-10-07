package collab

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// Hub routes the messages between the clients connected to the same server instance.
//
// PostgreSQL serializes mutations; Redis distributes notifications. Notifications
// may be missing or reordered even on this instance; clients recover them from
// the persistent changesets.
type Hub struct {
	backgroundContext context.Context
	cancelBackground  context.CancelFunc
	db                *gorm.DB

	mu       sync.Mutex
	projects map[string]map[*Client]struct{}

	shortcuts   ShortcutHooks
	cluster     *cluster
	draining    bool
	connections sync.WaitGroup
	background  sync.WaitGroup
}

// SetShortcutHooks connects the field shortcuts, it must be called before serving.
func (h *Hub) SetShortcutHooks(hooks ShortcutHooks) {
	h.shortcuts = hooks
}

func NewHub(gormDB *gorm.DB) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		db:                gormDB,
		backgroundContext: ctx,
		cancelBackground:  cancel,
		projects:          make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) register(c *Client) bool {
	h.mu.Lock()
	if h.draining {
		h.mu.Unlock()
		return false
	}

	h.connections.Add(1)
	clients, ok := h.projects[c.projectUID]
	if !ok {
		clients = make(map[*Client]struct{})
		h.projects[c.projectUID] = clients
	}
	clients[c] = struct{}{}
	h.mu.Unlock()

	c.send(newMessage(MessageHello, 0, helloData{ClientID: c.member.ClientID}))
	h.broadcastMembers(c.projectUID)
	h.publish(clusterEvent{Kind: clusterEventPresenceRequest, ProjectUID: c.projectUID})
	return true
}

func (h *Hub) unregister(c *Client) {
	defer h.connections.Done()

	h.mu.Lock()
	delete(h.projects[c.projectUID], c)
	if len(h.projects[c.projectUID]) == 0 {
		delete(h.projects, c.projectUID)
	}
	h.mu.Unlock()

	h.broadcastMembers(c.projectUID)
}

func (h *Hub) clients(projectUID string) []*Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := make([]*Client, 0, len(h.projects[projectUID]))
	for c := range h.projects[projectUID] {
		clients = append(clients, c)
	}

	return clients
}

func (h *Hub) broadcastMembers(projectUID string) {
	// Assign a sequence number under the snapshot lock to reject delayed updates.
	h.mu.Lock()
	clients := h.projects[projectUID]
	members := make([]Member, 0, len(clients))
	for c := range clients {
		members = append(members, c.presence())
	}

	var version uint64
	if h.cluster != nil {
		version = h.cluster.version.Add(1)
	}
	h.mu.Unlock()

	h.deliverMembers(projectUID)
	h.publish(clusterEvent{Kind: clusterEventMembers, ProjectUID: projectUID, Members: members, Version: version})
}

func (h *Hub) deliverMembers(projectUID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	members := make([]Member, 0, len(h.projects[projectUID]))
	for c := range h.projects[projectUID] {
		members = append(members, c.presence())
	}

	if h.cluster != nil {
		for node, presence := range h.cluster.members[projectUID] {
			if time.Now().After(presence.expires) {
				delete(h.cluster.members[projectUID], node)
				continue
			}
			members = append(members, presence.members...)
		}
	}

	sort.Slice(members, func(i, j int) bool { return members[i].ClientID < members[j].ClientID })
	message := newMessage(MessageMembers, 0, membersData{Members: members})
	for c := range h.projects[projectUID] {
		c.send(message)
	}
}

// UpdateUserAvatar refreshes all connections of the user, ignoring an older concurrent update.
func (h *Hub) UpdateUserAvatar(userID int64, avatarURL string, updatedAt time.Time) {
	h.broadcastEvent(clusterEvent{Kind: clusterEventAvatar, UserID: userID, AvatarURL: avatarURL, UpdatedAt: updatedAt})
}

// subscribe subscribes the client to the table changes, and returns the latest revision of the table.
func (h *Hub) subscribe(ctx context.Context, c *Client, table *db.SLTable) (int64, error) {
	// Subscribe first: a commit racing the revision read is either queued
	// live or included in the revision returned for HTTP catch-up.
	c.subscribe(table.UID)
	store := db.SLTables
	if h.db != nil {
		store = db.NewSLTablesStore(h.db)
	}
	latest, err := store.GetByID(ctx, table.ID)
	if err != nil {
		return 0, errors.Wrap(err, "get table")
	}

	return latest.Rev, nil
}

// CommitResult is the result of committing a changeset.
type CommitResult struct {
	Rev int64
	// Duplicate reports the changeset with the same signature has been committed before, e.g. resent after reconnecting.
	Duplicate bool
}

// Commit applies the operations to the table and broadcasts the accepted changeset to the subscribers except the sender.
// If the server changes other data when applying, e.g. converting the values of a field, a server changeset with the dirty
// scope is appended in the same transaction and broadcast to all the subscribers.
//
// onAccept runs before this commit's broadcasts. Concurrent commits may notify
// subscribers in any revision order.
// It returns *OperationError if the changeset is rejected.
func (h *Hub) Commit(ctx context.Context, project *db.Project, table *db.SLTable, sender *Client, signature string, operations []Operation, onAccept func(CommitResult)) error {
	_, err := h.commit(ctx, project, table, sender, signature, operations, nil, onAccept)
	return err
}

// CommitServer applies the operations generated by the server, e.g. writing back the results of the field shortcuts, and broadcasts
// them to all the subscribers. guard runs first in the same transaction, the changeset is skipped if it returns false.
// It reports whether the changeset is committed.
func (h *Hub) CommitServer(ctx context.Context, project *db.Project, table *db.SLTable, operations []Operation, guard func(tx *gorm.DB) (bool, error)) (bool, error) {
	return h.commit(ctx, project, table, nil, "", operations, guard, func(CommitResult) {})
}

func (h *Hub) commit(ctx context.Context, project *db.Project, table *db.SLTable, sender *Client, signature string, operations []Operation, guard func(tx *gorm.DB) (bool, error), onAccept func(CommitResult)) (bool, error) {
	var clientID string
	if sender != nil {
		clientID = sender.member.ClientID
	}

	var changesets []*Changeset
	var change Change
	errSkipped := errors.New("skipped by the guard")
	var duplicate *db.SLChangeset
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := dbutil.LockTable(ctx, tx, table.ID); err != nil {
			return err
		}

		// Check inside the shared lock, including when another node accepted a
		// retry while this transaction was waiting.
		if signature != "" {
			item, err := db.NewSLChangesetsStore(tx).GetBySignature(ctx, table.ID, signature)
			if err == nil {
				duplicate = item
				return nil
			}
			if !errors.Is(err, db.ErrSLChangesetNotFound) {
				return err
			}
		}

		if sender != nil {
			role, err := sender.currentRole(ctx, tx)
			if err != nil || !role.AtLeast(db.ProjectRoleEditor) {
				return rejectf("collab::no_edit_permission")
			}
		}

		if guard != nil {
			ok, err := guard(tx)
			if err != nil {
				return err
			}
			if !ok {
				return errSkipped
			}
		}

		a, err := newApplier(ctx, tx, table, sender.boolText, h.shortcuts)
		if err != nil {
			return err
		}
		applied, err := a.apply(operations)
		if err != nil {
			return err
		}

		changeset, err := appendChangeset(ctx, tx, table, signature, clientID, applied)
		if err != nil {
			return err
		}
		changesets = append(changesets, changeset)

		if !a.dirty.IsEmpty() {
			dirty := a.dirty
			changeset, err := appendChangeset(ctx, tx, table, "", "", []Operation{{
				Command: "Refresh",
				Actions: []Action{{Action: ActionDirty, Dirty: &dirty}},
			}})
			if err != nil {
				return err
			}
			changesets = append(changesets, changeset)
		}

		// The tables without shortcuts have nothing to regenerate.
		if a.hasShortcuts() || len(a.change.RemovedShortcuts) > 0 {
			change = a.change
		}

		return nil
	}); err != nil {
		if errors.Is(err, errSkipped) {
			return false, nil
		}
		return false, err
	}

	if duplicate != nil {
		onAccept(CommitResult{Rev: duplicate.Rev, Duplicate: true})
		return false, nil
	}
	onAccept(CommitResult{Rev: changesets[0].Rev})
	for i, changeset := range changesets {
		h.broadcastChangeset(project, changeset, ternary(i == 0, sender, nil))
	}

	h.NotifyChange(ctx, project, table, &change)
	h.notifyOptionReferences(ctx, project, table, operations)

	return true, nil
}

// NotifyChange tells the field shortcuts the data is changed, it runs in the background so the caller is not blocked.
func (h *Hub) NotifyChange(ctx context.Context, project *db.Project, table *db.SLTable, change *Change) {
	if h.shortcuts == nil || change.IsEmpty() {
		return
	}

	h.runBackground(ctx, func(ctx context.Context) { h.shortcuts.ShortcutsChanged(ctx, project, table, change) })
}

// NotifyTable sends the message to the subscribers of the table, build returns the data in the language of each subscriber.
func (h *Hub) NotifyTable(projectUID, tableUID, messageType string, build func(tr func(key string, args ...interface{}) string) interface{}) {
	for _, c := range h.clients(projectUID) {
		if c.subscribed(tableUID) {
			c.send(newMessage(messageType, 0, build(c.tr)))
		}
	}

	// Job status messages contain localized errors. Remote clients reload the
	// authoritative queue, translating it in their own request locale.
	h.publish(clusterEvent{Kind: clusterEventTable, ProjectUID: projectUID, TableUID: tableUID, MessageType: messageType})
}

func appendChangeset(ctx context.Context, tx *gorm.DB, table *db.SLTable, signature, clientID string, operations []Operation) (*Changeset, error) {
	rev, err := db.NewSLTablesStore(tx).IncreaseRev(ctx, table.ID)
	if err != nil {
		return nil, errors.Wrap(err, "increase rev")
	}
	raw, err := json.Marshal(operations)
	if err != nil {
		return nil, errors.Wrap(err, "encode operations")
	}
	if _, err := db.NewSLChangesetsStore(tx).Create(ctx, db.CreateSLChangesetOptions{
		SLTableID:  table.ID,
		Rev:        rev,
		Signature:  signature,
		ClientID:   clientID,
		Operations: raw,
	}); err != nil {
		return nil, errors.Wrap(err, "create changeset")
	}
	return &Changeset{
		TableUID:   table.UID,
		Rev:        rev,
		Signature:  signature,
		ClientID:   clientID,
		Operations: operations,
	}, nil
}

func (h *Hub) broadcastChangeset(project *db.Project, changeset *Changeset, except *Client) {
	h.deliverChangeset(project.UID, changeset, except)
	h.publish(clusterEvent{Kind: clusterEventChanges, ProjectUID: project.UID, Changeset: changeset})
}

func (h *Hub) deliverChangeset(projectUID string, changeset *Changeset, except *Client) {
	message := newMessage(MessageNewChanges, 0, changeset)
	for _, c := range h.clients(projectUID) {
		if c != except && c.subscribed(changeset.TableUID) {
			c.send(message)
		}
	}
}

// NotifyProject tells the clients of the project to reload the project or its table list.
func (h *Hub) NotifyProject(projectUID, messageType string) {
	h.broadcastEvent(clusterEvent{Kind: clusterEventProject, ProjectUID: projectUID, MessageType: messageType})
}

// SetUserRole applies the new role of the user to its connections of the project and notifies them.
// An empty role means the user is removed from the project, and the connections are closed after the notification.
func (h *Hub) SetUserRole(projectUID string, userID int64, role db.ProjectRole) {
	message := newMessage(MessagePermissionChanged, 0, permissionChangedData{ProjectUID: projectUID, Role: string(role)})
	for _, c := range h.clients(projectUID) {
		if c.userID != userID || c.share != nil {
			continue
		}

		c.mu.Lock()
		c.role = role
		c.mu.Unlock()

		c.send(message)
		if role == "" {
			c.close()
		}
	}

	h.publish(clusterEvent{Kind: clusterEventRole, ProjectUID: projectUID, UserID: userID})
}

// DisconnectUser closes the connections of the user on all projects.
func (h *Hub) DisconnectUser(userID int64) {
	h.broadcastEvent(clusterEvent{Kind: clusterEventDisconnect, UserID: userID})
}

// Mutate commits a REST mutation and its invalidation in the same transaction.
// It shares the PostgreSQL row lock with WebSocket and worker writes.
func (h *Hub) Mutate(ctx context.Context, project *db.Project, table *db.SLTable, mutate func(*gorm.DB) error, scope func() DirtyScope) error {
	var changeset *Changeset
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := dbutil.LockTable(ctx, tx, table.ID); err != nil {
			return err
		}
		if err := mutate(tx); err != nil {
			return err
		}

		dirty := scope()
		var err error
		changeset, err = appendChangeset(ctx, tx, table, "", "", []Operation{{
			Command: "Refresh", Actions: []Action{{Action: ActionDirty, Dirty: &dirty}},
		}})
		return err
	}); err != nil {
		return err
	}

	h.broadcastChangeset(project, changeset, nil)
	h.notifyOptionReferences(ctx, project, table, changeset.Operations)

	return nil
}

func ternary[T any](condition bool, a, b T) T {
	if condition {
		return a
	}
	return b
}
