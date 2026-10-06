package shortcut

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
)

// runJob owns one fencing token. Renewing a lost lease cancels execution;
// completion and cleanup also check the token, so neither can affect a successor.
func (e *Engine) runJob(ctx context.Context, job *db.SLShortcutJob) {
	execution, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)

		ticker := time.NewTicker(leaseInterval)
		defer ticker.Stop()

		for {
			select {
			case <-execution.Done():
				return
			case <-ticker.C:
				renewal, stop := context.WithTimeout(execution, cleanupTimeout)
				owned, err := e.jobs().Renew(renewal, job)
				stop()
				if err != nil || !owned {
					cancel()
					return
				}
			}
		}
	}()

	e.process(execution, job)
	cancel()
	<-done

	cleanup, stop := context.WithTimeout(context.Background(), cleanupTimeout)
	defer stop()

	if _, err := e.jobs().Release(cleanup, job); err != nil {
		e.stopMu.Lock()
		e.releaseErr = err
		e.stopMu.Unlock()

		logrus.WithError(err).WithField("jobID", job.ID).Error("Failed to release interrupted shortcut job")
	}
}
