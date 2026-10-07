package collab

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BeginDrain refuses new WebSocket handshakes, including upgrades racing shutdown.
func (h *Hub) BeginDrain() {
	h.mu.Lock()
	h.draining = true
	h.mu.Unlock()
}

func (h *Hub) closeConnections(code int) {
	h.mu.Lock()
	clients := make([]*Client, 0)
	for _, project := range h.projects {
		for c := range project {
			clients = append(clients, c)
		}
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.mu.Lock()
		c.closeStatus = code
		c.mu.Unlock()

		c.close()
		if c.conn != nil {
			// WriteControl is safe alongside writePump. Close even if the peer
			// ignores the close handshake, so readPump and its handler can exit.
			go func() {
				_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, ""), time.Now().Add(time.Second))
				_ = c.conn.Close()
			}()
		}
	}
}

func waitGroup(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown closes sockets with code 1012 (service restart) and waits for their
// in-flight handlers. HTTP handlers must be drained separately by http.Server.
func (h *Hub) Shutdown(ctx context.Context) error {
	h.BeginDrain()
	h.closeConnections(websocket.CloseServiceRestart)

	return waitGroup(ctx, &h.connections)
}

func (h *Hub) runBackground(parent context.Context, fn func(context.Context)) {
	h.background.Go(func() {
		ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
		stop := context.AfterFunc(h.backgroundContext, cancel)
		defer stop()
		defer cancel()

		fn(ctx)
	})
}

// WaitBackground is called once HTTP handlers, WebSocket handlers and workers
// can no longer create new tasks. Tasks may create children before returning.
func (h *Hub) WaitBackground(ctx context.Context) error {
	return waitGroup(ctx, &h.background)
}

func (h *Hub) CancelBackground() { h.cancelBackground() }
