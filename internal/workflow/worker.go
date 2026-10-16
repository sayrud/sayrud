package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

const (
	jobMaxAttempts    = 3
	jobPollInterval   = 2 * time.Second
	jobStaleAfter     = 60 * time.Second
	jobLeaseInterval  = 15 * time.Second
	jobCleanupTimeout = 5 * time.Second
	// Runs and finished jobs older than runRetention are cleaned up.
	runRetention    = 30 * 24 * time.Hour
	retentionPeriod = time.Hour
)

// jobRetryDelays are the delays before the second and third attempts.
var jobRetryDelays = []time.Duration{10 * time.Second, time.Minute}

var jobTimeout = 5 * time.Minute

var errLeaseLost = errors.New("the job lease is lost")

// Start starts the background workers, which run until claiming stops; Shutdown drains or cancels the running jobs.
func (e *Engine) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		claims, stopClaims := context.WithCancel(ctx)
		execution, cancelExecution := context.WithCancel(context.WithoutCancel(ctx))

		e.stopMu.Lock()
		e.stopClaims, e.cancelExecution = stopClaims, cancelExecution
		e.stopMu.Unlock()

		for range e.workers {
			e.wg.Go(func() { e.work(claims, execution) })
		}
		e.wg.Go(func() { e.maintain(claims) })
		e.wg.Go(func() { e.scheduleLoop(claims) })

		go func() {
			e.wg.Wait()
			cancelExecution()
			close(e.done)
		}()
	})
}

// Stop stops claiming new jobs without canceling the running ones.
func (e *Engine) Stop() {
	e.stopMu.Lock()
	defer e.stopMu.Unlock()
	if e.stopClaims != nil {
		e.stopClaims()
	}
}

// Shutdown drains the running jobs until grace is done, then cancels them and releases their leases within cleanup.
func (e *Engine) Shutdown(grace, cleanup context.Context) error {
	e.Stop()
	e.stopMu.Lock()
	started := e.stopClaims != nil
	e.stopMu.Unlock()
	if !started {
		return nil
	}
	select {
	case <-e.done:
		return e.releaseErr
	case <-grace.Done():
	}
	e.cancelExecution()
	select {
	case <-e.done:
		return e.releaseErr
	case <-cleanup.Done():
		return cleanup.Err()
	}
}

func (e *Engine) jobs() db.WorkflowJobsStore {
	return db.NewWorkflowJobsStore(e.db)
}

func (e *Engine) notifyWorkers() {
	for range e.workers {
		select {
		case e.wake <- struct{}{}:
		default:
			return
		}
	}
}

func (e *Engine) work(ctx, execution context.Context) {
	for ctx.Err() == nil {
		job, err := e.jobs().Claim(ctx)
		if err != nil && ctx.Err() == nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to claim workflow job")
		}
		if job != nil {
			if ctx.Err() != nil {
				e.releaseJob(job)
				return
			}
			e.runJob(execution, job)
			continue
		}
		select {
		case <-ctx.Done():
		case <-e.wake:
		case <-time.After(jobPollInterval):
		}
	}
}

func (e *Engine) maintain(ctx context.Context) {
	ticker := time.NewTicker(jobLeaseInterval)
	defer ticker.Stop()
	lastRetention := time.Time{}
	for {
		if n, failed, err := e.jobs().ResetStale(ctx, jobStaleAfter, jobMaxAttempts); err != nil {
			if ctx.Err() == nil {
				logrus.WithContext(ctx).WithError(err).Error("Failed to reset stale workflow jobs")
			}
		} else {
			if n > 0 {
				logrus.WithContext(ctx).WithField("count", n).Warn("Reset stale workflow jobs")
				e.notifyWorkers()
			}
			for _, job := range failed {
				logrus.WithContext(ctx).WithField("jobID", job.ID).Warn("Failed workflow job interrupted repeatedly")
				var trace []TraceStep
				_ = json.Unmarshal(job.Trace, &trace)
				e.finishRun(ctx, job, db.WorkflowRunFailed, trace, map[string]interface{}{"message": db.WorkflowJobInterruptedMessage}, job.CreatedAt)
			}
		}
		if time.Since(lastRetention) > retentionPeriod {
			lastRetention = time.Now()
			if _, err := db.NewWorkflowRunsStore(e.db).DeleteBefore(ctx, dbutil.Now().Add(-runRetention)); err != nil && ctx.Err() == nil {
				logrus.WithContext(ctx).WithError(err).Error("Failed to delete old workflow runs")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// runJob holds a lease token: the execution is canceled as soon as a renewal fails, and saving and finishing both verify the token so they never affect a worker that took over.
func (e *Engine) runJob(ctx context.Context, job *db.WorkflowJob) {
	execution, cancel := context.WithTimeout(ctx, jobTimeout)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(jobLeaseInterval)
		defer ticker.Stop()
		for {
			select {
			case <-execution.Done():
				return
			case <-ticker.C:
				renewal, stop := context.WithTimeout(execution, jobCleanupTimeout)
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
	e.releaseJob(job)
}

func (e *Engine) releaseJob(job *db.WorkflowJob) {
	cleanup, stop := context.WithTimeout(context.Background(), jobCleanupTimeout)
	defer stop()
	if _, err := e.jobs().Release(cleanup, job); err != nil {
		e.stopMu.Lock()
		e.releaseErr = err
		e.stopMu.Unlock()
		logrus.WithError(err).WithField("jobID", job.ID).Error("Failed to release interrupted workflow job")
	}
}

func (e *Engine) process(ctx context.Context, job *db.WorkflowJob) {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{"jobID": job.ID, "workflowID": job.WorkflowID})
	start := time.Now()
	var payload jobPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		e.failJob(ctx, job, "the job payload is invalid", false, nil, start)
		return
	}
	project, err := db.NewProjectsStore(e.db).GetByID(ctx, payload.ProjectID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			e.failJob(ctx, job, "the project has been deleted", false, nil, start)
			return
		}
		e.retryJob(ctx, job, "internal error", nil, start)
		return
	}
	steps, err := e.jobSteps(ctx, job, &payload)
	if err != nil {
		if errors.Is(err, errFlowNotFound) {
			e.failJob(ctx, job, "the flow does not exist in the version", false, nil, start)
			return
		}
		e.retryJob(ctx, job, "internal error", nil, start)
		return
	}
	if job.FlowID == MainFlow {
		if err := db.NewWorkflowRunsStore(e.db).Start(ctx, job.RunID); err != nil {
			log.WithError(err).Error("Failed to start workflow run")
		}
	}

	state := map[string]json.RawMessage{}
	_ = json.Unmarshal(job.State, &state)
	scope := payload.Scope
	if scope == nil {
		scope = Scope{}
	}
	if _, ok := scope["steps"].(map[string]interface{}); !ok {
		scope["steps"] = map[string]interface{}{}
	}

	x := &execution{
		ctx: ctx, db: e.db, hub: e.hub, project: project, scope: scope, tables: map[string]*tableInfo{}, actions: e.actions, done: state,
		origin: collab.Origin{WorkflowID: job.WorkflowID, Depth: payload.Depth},
	}
	x.persist = func(tx *gorm.DB, stepID string, output interface{}) error {
		state[stepID] = marshalJSON(output)
		store := e.jobs()
		if tx != nil {
			store = db.NewWorkflowJobsStore(tx)
		}
		owned, err := store.SaveState(ctx, job, marshalJSON(state), marshalJSON(x.trace))
		if err != nil {
			return err
		}
		if !owned {
			return errLeaseLost
		}
		return nil
	}

	_, err = x.runSteps(steps)
	if err == nil {
		owned, err := e.jobs().Succeed(ctx, job, marshalJSON(x.trace))
		if err != nil {
			log.WithError(err).Error("Failed to finish workflow job")
			return
		}
		// A worker that took over the lost lease finishes the run instead.
		if owned {
			e.finishRun(ctx, job, db.WorkflowRunSucceeded, x.trace, nil, start)
		}
		return
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, errLeaseLost) {
		// runJob releases the interrupted job so that another worker can claim it.
		return
	}
	if ctx.Err() != nil {
		// Exceeding jobTimeout counts as a failed attempt; the context has expired, so the outcome is saved without it.
		ctx = context.WithoutCancel(ctx)
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) {
		stepErr = &StepError{Err: err, Transient: true}
	}
	message := publicError(stepErr)
	if stepErr.StepID != "" {
		message = stepErr.StepID + ": " + message
	}
	if stepErr.Transient && !stepErr.Uncertain && job.Attempts < jobMaxAttempts {
		e.retryJob(ctx, job, message, x.trace, start)
		return
	}
	e.failJob(ctx, job, message, stepErr.Uncertain, x.trace, start)
}

var errFlowNotFound = errors.New("flow not found")

// jobSteps returns the steps the job runs: saved in the job when debugging a draft, otherwise read from the job's revision.
func (e *Engine) jobSteps(ctx context.Context, job *db.WorkflowJob, payload *jobPayload) ([]Step, error) {
	if payload.Flow != nil {
		return payload.Flow.Steps, nil
	}
	spec, err := e.PublishedSpec(ctx, job.RevisionID)
	if err != nil {
		if errors.Is(err, db.ErrWorkflowRevisionNotFound) {
			return nil, errFlowNotFound
		}
		return nil, err
	}
	if job.FlowID == MainFlow {
		return spec.Steps, nil
	}
	for i := range spec.Async {
		if spec.Async[i].ID == job.FlowID {
			return spec.Async[i].Steps, nil
		}
	}
	return nil, errFlowNotFound
}

// finishRun updates the run when a main flow job finishes; jobs of async flows do not affect the run.
func (e *Engine) finishRun(ctx context.Context, job *db.WorkflowJob, status db.WorkflowRunStatus, trace []TraceStep, errBody interface{}, start time.Time) {
	if job.FlowID != MainFlow {
		return
	}
	if err := db.NewWorkflowRunsStore(e.db).Finish(context.WithoutCancel(ctx), job.RunID, db.FinishWorkflowRunOptions{
		Status: status, DurationMs: time.Since(start).Milliseconds(), Trace: marshalJSON(nonNilTrace(trace)), Error: marshalJSON(errBody),
	}); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to finish workflow run")
	}
}

func (e *Engine) retryJob(ctx context.Context, job *db.WorkflowJob, message string, trace []TraceStep, start time.Time) {
	if job.Attempts >= jobMaxAttempts {
		e.failJob(ctx, job, message, false, trace, start)
		return
	}
	delay := jobRetryDelays[min(max(job.Attempts, 1), len(jobRetryDelays))-1]
	if _, err := e.jobs().Retry(ctx, job, dbutil.Now().Add(delay), message, marshalJSON(nonNilTrace(trace))); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to retry workflow job")
	}
}

func (e *Engine) failJob(ctx context.Context, job *db.WorkflowJob, message string, uncertain bool, trace []TraceStep, start time.Time) {
	owned, err := e.jobs().Fail(ctx, job, message, uncertain, marshalJSON(nonNilTrace(trace)))
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to fail workflow job")
		return
	}
	if owned {
		e.finishRun(ctx, job, db.WorkflowRunFailed, trace, map[string]interface{}{"message": message}, start)
	}
}

func nonNilTrace(trace []TraceStep) []TraceStep {
	if trace == nil {
		return []TraceStep{}
	}
	return trace
}

// Requeue reruns a failed job without repeating steps that already succeeded; for a main flow job, the run is also put back in the queue.
func (e *Engine) Requeue(ctx context.Context, workflowID, jobID int64) (bool, error) {
	job, err := e.jobs().Requeue(ctx, workflowID, jobID)
	if err != nil || job == nil {
		return false, err
	}
	if job.FlowID == MainFlow {
		if err := db.NewWorkflowRunsStore(e.db).Requeue(ctx, job.RunID); err != nil {
			return false, err
		}
	}
	e.notifyWorkers()
	return true, nil
}
