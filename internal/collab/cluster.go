package collab

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
)

const (
	clusterChannel   = "sayrud:collab:v1"
	presenceInterval = 5 * time.Second
	presenceTTL      = 20 * time.Second
	clusterTimeout   = 2 * time.Second
)

// Cluster event kinds are shared by publishers and subscribers.
const (
	clusterEventChanges         = "changes"
	clusterEventMembers         = "members"
	clusterEventPresenceRequest = "presence-request"
	clusterEventProject         = "project"
	clusterEventRole            = "role"
	clusterEventDisconnect      = "disconnect"
	clusterEventAvatar          = "avatar"
	clusterEventShare           = "share"
	clusterEventTable           = "table"
)

// Events are hints. Changesets and permissions remain authoritative in PostgreSQL.
// Presence snapshots have node-local sequence numbers so delayed publishers cannot
// overwrite a newer cursor or resurrect a disconnected member.
type clusterEvent struct {
	Source      string
	Kind        string
	ProjectUID  string
	TableUID    string
	MessageType string
	UserID      int64
	TableID     int64
	AvatarURL   string
	UpdatedAt   time.Time
	Members     []Member
	Version     uint64
	Changeset   *Changeset
}

type remotePresence struct {
	members []Member
	version uint64
	expires time.Time
}

type cluster struct {
	hub     *Hub
	client  *redis.Client
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	ready   atomic.Bool
	version atomic.Uint64
	mu      sync.Mutex
	sub     *redis.PubSub
	wg      sync.WaitGroup

	// Protected by hub.mu; only projects with local subscribers are retained.
	members map[string]map[string]remotePresence
}

// StartCluster subscribes before any connections or workers are accepted.
func (h *Hub) StartCluster(ctx context.Context, client *redis.Client) error {
	ctx, cancel := context.WithCancel(ctx)
	c := &cluster{
		hub:     h,
		client:  client,
		id:      randstr.Hex(32),
		ctx:     ctx,
		cancel:  cancel,
		members: make(map[string]map[string]remotePresence),
	}
	if err := c.subscribe(); err != nil {
		cancel()
		return err
	}

	h.cluster = c
	c.wg.Go(c.receive)
	c.wg.Go(c.maintain)

	return nil
}

func (c *cluster) subscribe() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ctx.Err(); err != nil {
		return err
	}

	sub := c.client.Subscribe(c.ctx, clusterChannel)
	if _, err := sub.ReceiveTimeout(c.ctx, clusterTimeout); err != nil {
		_ = sub.Close()
		return err
	}

	c.sub = sub
	c.ready.Store(true)

	return nil
}

func (h *Hub) publish(event clusterEvent) {
	c := h.cluster
	if c == nil || c.ctx.Err() != nil {
		return
	}

	event.Source = c.id
	raw, err := json.Marshal(event)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode collaboration event")
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, clusterTimeout)
	defer cancel()

	if err := c.client.Publish(ctx, clusterChannel, raw).Err(); err != nil {
		logrus.WithError(err).Warn("Failed to publish collaboration event")
		c.failed()
	}
}

func (c *cluster) failed() {
	c.ready.Store(false)

	c.mu.Lock()
	if c.sub != nil {
		_ = c.sub.Close()
	}
	c.mu.Unlock()

	// A broken subscription may have missed permission changes as well as data.
	// Reconnecting sockets reauthenticate and catch up using persistent revisions.
	c.hub.closeConnections(websocket.CloseServiceRestart)
}

func (c *cluster) receive() {
	for c.ctx.Err() == nil {
		c.mu.Lock()
		sub := c.sub
		c.mu.Unlock()

		message, err := sub.Receive(c.ctx)
		if err != nil {
			c.failed()
			for c.ctx.Err() == nil {
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(time.Second):
				}
				if c.subscribe() == nil {
					break
				}
			}
			continue
		}

		switch message := message.(type) {
		case *redis.Subscription:
			// go-redis can reconnect internally; a subscribe acknowledgement means
			// the old subscription had a gap even when no error reached us.
			c.hub.closeConnections(websocket.CloseServiceRestart)

		case *redis.Message:
			var event clusterEvent
			if json.Unmarshal([]byte(message.Payload), &event) == nil && event.Source != c.id {
				c.hub.receiveEvent(event)
			}
		}
	}
}

func (c *cluster) maintain() {
	ticker := time.NewTicker(presenceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, clusterTimeout)
			err := c.client.Ping(ctx).Err()
			cancel()
			if err != nil {
				c.failed()
				continue
			}

			c.hub.mu.Lock()
			projects := make([]string, 0, len(c.hub.projects))
			for projectUID := range c.hub.projects {
				projects = append(projects, projectUID)
			}
			for projectUID := range c.members {
				if len(c.hub.projects[projectUID]) == 0 {
					delete(c.members, projectUID)
				}
			}
			c.hub.mu.Unlock()

			for _, projectUID := range projects {
				c.hub.broadcastMembers(projectUID)
			}
		}
	}
}

// broadcastEvent applies a local notification before publishing it. Received
// events only enter receiveEvent, so they cannot be republished in a loop.
func (h *Hub) broadcastEvent(event clusterEvent) {
	h.receiveEvent(event)
	h.publish(event)
}

func (h *Hub) receiveEvent(event clusterEvent) {
	switch event.Kind {
	case clusterEventChanges:
		if event.Changeset != nil {
			h.deliverChangeset(event.ProjectUID, event.Changeset, nil)
		}

	case clusterEventMembers:
		h.mu.Lock()
		if len(h.projects[event.ProjectUID]) == 0 {
			h.mu.Unlock()
			return
		}

		nodes := h.cluster.members[event.ProjectUID]
		if nodes == nil {
			nodes = make(map[string]remotePresence)
			h.cluster.members[event.ProjectUID] = nodes
		}

		if previous, ok := nodes[event.Source]; !ok || event.Version > previous.version {
			nodes[event.Source] = remotePresence{members: event.Members, version: event.Version, expires: time.Now().Add(presenceTTL)}
		}
		h.mu.Unlock()
		h.deliverMembers(event.ProjectUID)

	case clusterEventPresenceRequest:
		if len(h.clients(event.ProjectUID)) > 0 {
			h.broadcastMembers(event.ProjectUID)
		}

	case clusterEventProject:
		message := newMessage(event.MessageType, 0, projectEventData{ProjectUID: event.ProjectUID})
		for _, c := range h.clients(event.ProjectUID) {
			c.send(message)
		}

	case clusterEventRole:
		for _, c := range h.clients(event.ProjectUID) {
			if c.userID == event.UserID && c.share == nil {
				ctx, cancel := context.WithTimeout(h.cluster.ctx, clusterTimeout)
				c.refreshAccess(ctx)
				cancel()
			}
		}

	case clusterEventDisconnect:
		h.mu.Lock()
		var targets []*Client
		for _, clients := range h.projects {
			for c := range clients {
				if c.userID == event.UserID {
					targets = append(targets, c)
				}
			}
		}
		h.mu.Unlock()

		for _, c := range targets {
			c.close()
		}

	case clusterEventAvatar:
		var projects []string

		h.mu.Lock()
		for projectUID, clients := range h.projects {
			changed := false
			for c := range clients {
				if c.userID != event.UserID {
					continue
				}

				c.mu.Lock()
				if !c.avatarUpdatedAt.After(event.UpdatedAt) {
					c.member.AvatarURL = event.AvatarURL
					c.avatarUpdatedAt = event.UpdatedAt
					changed = true
				}
				c.mu.Unlock()
			}

			if changed {
				projects = append(projects, projectUID)
			}
		}
		h.mu.Unlock()

		for _, projectUID := range projects {
			h.broadcastMembers(projectUID)
		}

	case clusterEventShare:
		for _, c := range h.clients(event.ProjectUID) {
			if c.share != nil && c.share.TableID == event.TableID {
				c.send(newMessage(MessageShareChanged, 0, nil))
				c.close()
			}
		}

	case clusterEventTable:
		for _, c := range h.clients(event.ProjectUID) {
			if c.subscribed(event.TableUID) {
				c.send(newMessage(event.MessageType, 0, map[string]interface{}{"tableUID": event.TableUID, "reload": true}))
			}
		}
	}
}

func (h *Hub) ClusterReady() bool {
	return h.cluster == nil || h.cluster.ready.Load()
}

// StopCluster is called after requests, sockets, workers and follow-up tasks end.
func (h *Hub) StopCluster() {
	if h.cluster == nil {
		return
	}

	c := h.cluster
	c.cancel()

	c.mu.Lock()
	_ = c.sub.Close()
	c.mu.Unlock()

	c.wg.Wait()
}
