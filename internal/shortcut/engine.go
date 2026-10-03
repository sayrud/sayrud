package shortcut

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
)

const (
	maxAttempts      = 3
	pollInterval     = 2 * time.Second
	staleAfter       = 20 * time.Minute
	maxBroadcastJobs = 200
	// jobTimeoutMargin is added to the timeout of the executor, so the executor reports its own timeout first.
	jobTimeoutMargin = 10 * time.Second
)

// retryDelays are the delays before the second and the third attempts.
var retryDelays = []time.Duration{5 * time.Second, 30 * time.Second}

var _ collab.ShortcutHooks = (*Engine)(nil)

// Engine queues the cells to generate and executes the jobs in the background, it writes the results back as server changesets.
type Engine struct {
	db       *gorm.DB
	hub      *collab.Hub
	executor *Executor
	workers  int
	wake     chan struct{}
}

// NewEngine returns the engine connected to the hub, the jobs are executed after Start.
func NewEngine(gormDB *gorm.DB, hub *collab.Hub, executor *Executor, workers int) *Engine {
	e := &Engine{
		db:       gormDB,
		hub:      hub,
		executor: executor,
		workers:  max(workers, 1),
		wake:     make(chan struct{}, max(workers, 1)),
	}
	hub.SetShortcutHooks(e)
	return e
}

// Start runs the workers until the context is canceled.
func (e *Engine) Start(ctx context.Context) {
	for range e.workers {
		go e.work(ctx)
	}
	go e.maintain(ctx)
}

func (e *Engine) jobs() db.SLShortcutJobsStore {
	return db.NewSLShortcutJobsStore(e.db)
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

func (e *Engine) work(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := e.jobs().Claim(ctx)
		if err != nil {
			if ctx.Err() == nil {
				logrus.WithContext(ctx).WithError(err).Error("Failed to claim shortcut job")
			}
		}
		if job != nil {
			e.process(ctx, job)
			continue
		}

		select {
		case <-ctx.Done():
		case <-e.wake:
		case <-time.After(pollInterval):
		}
	}
}

// maintain resets the jobs left running by a crashed server.
func (e *Engine) maintain(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if n, err := e.jobs().ResetStale(ctx, dbutil.Now().Add(-staleAfter)); err != nil {
			if ctx.Err() == nil {
				logrus.WithContext(ctx).WithError(err).Error("Failed to reset stale shortcut jobs")
			}
		} else if n > 0 {
			logrus.WithContext(ctx).WithField("count", n).Warn("Reset stale shortcut jobs")
			e.notifyWorkers()
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// jobState is a job change to broadcast.
type jobState struct {
	fieldUID  string
	recordUID string
	status    string
	err       db.ShortcutJobError
}

const statusDone = "done"

func (e *Engine) broadcast(projectUID, tableUID string, states []jobState) {
	if len(states) == 0 {
		return
	}
	e.hub.NotifyTable(projectUID, tableUID, collab.MessageShortcutJobs, func(tr func(string, ...interface{}) string) interface{} {
		if len(states) > maxBroadcastJobs {
			return dto.ShortcutJobsMessage{TableUID: tableUID, Reload: true, Jobs: []*dto.ShortcutJob{}}
		}
		jobs := make([]*dto.ShortcutJob, 0, len(states))
		for _, s := range states {
			jobs = append(jobs, &dto.ShortcutJob{FieldUID: s.fieldUID, RecordUID: s.recordUID, Status: s.status, Error: ErrorText(tr, s.err)})
		}
		return dto.ShortcutJobsMessage{TableUID: tableUID, Jobs: jobs}
	})
}

func (e *Engine) broadcastReload(projectUID, tableUID string) {
	e.hub.NotifyTable(projectUID, tableUID, collab.MessageShortcutJobs, func(func(string, ...interface{}) string) interface{} {
		return dto.ShortcutJobsMessage{TableUID: tableUID, Reload: true, Jobs: []*dto.ShortcutJob{}}
	})
}

// Enqueue queues the cells of the field for generating and notifies the subscribers of the table.
func (e *Engine) Enqueue(ctx context.Context, project *db.Project, table *db.SLTable, fieldUID string, recordUIDs []string) error {
	if len(recordUIDs) == 0 {
		return nil
	}
	jobs, err := e.jobs().Enqueue(ctx, table.ID, fieldUID, recordUIDs)
	if err != nil {
		return errors.Wrap(err, "enqueue")
	}

	e.broadcast(project.UID, table.UID, lo.Map(jobs, func(j *db.SLShortcutJob, _ int) jobState {
		return jobState{fieldUID: j.FieldUID, recordUID: j.RecordUID, status: string(db.ShortcutJobPending)}
	}))
	e.notifyWorkers()
	return nil
}

// RunScope selects the cells to generate.
type RunScope string

const (
	RunScopeAll     RunScope = "all"
	RunScopeEmpty   RunScope = "empty"
	RunScopeRecords RunScope = "records"
)

// Run queues the cells of the field in the scope, recordUIDs are used by RunScopeRecords. It returns the number of the cells queued.
func (e *Engine) Run(ctx context.Context, project *db.Project, table *db.SLTable, field *db.SLField, scope RunScope, recordUIDs []string) (int, error) {
	records := db.NewSLRecordsStore(e.db)
	var uids []string
	var err error
	switch scope {
	case RunScopeEmpty:
		uids, err = records.ListUIDs(ctx, table.ID, field.UID)
	case RunScopeRecords:
		var list []*db.SLRecord
		list, err = records.ListByUIDs(ctx, table.ID, recordUIDs)
		uids = lo.Map(list, func(r *db.SLRecord, _ int) string { return r.UID })
	default:
		uids, err = records.ListUIDs(ctx, table.ID, "")
	}
	if err != nil {
		return 0, errors.Wrap(err, "list records")
	}
	return len(uids), e.Enqueue(ctx, project, table, field.UID, uids)
}

// ListJobs returns the jobs of the table with the errors in the language of the reader.
func (e *Engine) ListJobs(ctx context.Context, table *db.SLTable, tr Translator) ([]*dto.ShortcutJob, error) {
	jobs, err := e.jobs().ListByTableID(ctx, table.ID)
	if err != nil {
		return nil, err
	}
	return lo.Map(jobs, func(j *db.SLShortcutJob, _ int) *dto.ShortcutJob {
		return &dto.ShortcutJob{FieldUID: j.FieldUID, RecordUID: j.RecordUID, Status: string(j.Status), Error: ErrorText(tr, j.Error.Data())}
	}), nil
}

// ValidateShortcut implements collab.ShortcutHooks.
func (e *Engine) ValidateShortcut(ctx context.Context, fields []*db.SLField, field *db.SLField) (*db.FieldShortcut, error) {
	shortcut, err := ValidateField(ctx, fields, field)
	if err != nil {
		var shortcutErr *Error
		if errors.As(err, &shortcutErr) {
			args := lo.Map(shortcutErr.Args, func(arg string, _ int) interface{} {
				if messageKeyPattern.MatchString(arg) {
					return collab.MessageKey(arg)
				}
				return arg
			})
			return nil, collab.Reject(shortcutErr.Key, args...)
		}
		return nil, err
	}
	return shortcut, nil
}

// ValidateField looks up the shortcut of the field and validates it, it returns *Error if invalid.
// A disabled shortcut is still valid; its cells fail with the reason when generating.
func ValidateField(ctx context.Context, fields []*db.SLField, field *db.SLField) (*db.FieldShortcut, error) {
	def, err := Lookup(ctx, field.Shortcut.ID)
	if err != nil {
		return nil, err
	}
	return Validate(def, fields, field, field.Shortcut)
}

// ShortcutsChanged implements collab.ShortcutHooks, it queues the cells to regenerate by the change.
func (e *Engine) ShortcutsChanged(ctx context.Context, project *db.Project, table *db.SLTable, change *collab.Change) {
	if err := e.handleChange(ctx, project, table, change); err != nil {
		logrus.WithContext(ctx).WithError(err).WithField("tableUID", table.UID).Error("Failed to queue shortcut jobs")
	}
}

func (e *Engine) handleChange(ctx context.Context, project *db.Project, table *db.SLTable, change *collab.Change) error {
	if len(change.RemovedShortcuts) > 0 {
		e.broadcastReload(project.UID, table.UID)
	}

	fields, err := db.NewSLFieldsStore(e.db).ListByTableID(ctx, table.ID)
	if err != nil {
		return errors.Wrap(err, "list fields")
	}
	var allUIDs []string
	listAll := func() ([]string, error) {
		if allUIDs == nil {
			uids, err := db.NewSLRecordsStore(e.db).ListUIDs(ctx, table.ID, "")
			if err != nil {
				return nil, errors.Wrap(err, "list records")
			}
			allUIDs = uids
		}
		return allUIDs, nil
	}

	for _, field := range fields {
		if field.Shortcut == nil {
			continue
		}
		deps := Dependencies(field.Shortcut)
		var uids []string
		switch {
		case lo.Contains(change.Shortcuts, field.UID),
			field.Shortcut.AutoUpdate && lo.Some(change.Fields, deps):
			if uids, err = listAll(); err != nil {
				return err
			}
		case field.Shortcut.AutoUpdate:
			for recordUID, changed := range change.Records {
				if changed == nil || lo.Some(changed, deps) {
					uids = append(uids, recordUID)
				}
			}
		}

		if err := e.Enqueue(ctx, project, table, field.UID, uids); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) process(ctx context.Context, job *db.SLShortcutJob) {
	log := logrus.WithContext(ctx).WithFields(logrus.Fields{"tableID": job.SLTableID, "fieldUID": job.FieldUID, "recordUID": job.RecordUID})

	table, err := db.NewSLTablesStore(e.db).GetByID(ctx, job.SLTableID)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			e.drop(ctx, job)
			return
		}
		e.fail(ctx, nil, job, err)
		return
	}

	project, err := db.NewProjectsStore(e.db).GetByID(ctx, table.ProjectID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			e.drop(ctx, job)
			return
		}
		e.fail(ctx, nil, job, err)
		return
	}

	target := &jobTarget{project: project, table: table}
	e.broadcast(project.UID, table.UID, []jobState{{fieldUID: job.FieldUID, recordUID: job.RecordUID, status: string(db.ShortcutJobRunning)}})

	fields, err := db.NewSLFieldsStore(e.db).ListByTableID(ctx, table.ID)
	if err != nil {
		e.fail(ctx, target, job, err)
		return
	}
	field, ok := lo.Find(fields, func(f *db.SLField) bool { return f.UID == job.FieldUID })
	if !ok || field.Shortcut == nil {
		e.finish(ctx, target, job)
		return
	}

	record, err := db.NewSLRecordsStore(e.db).GetByUID(ctx, job.RecordUID)
	if err != nil || record.SLTableID != table.ID {
		if err == nil || errors.Is(err, db.ErrSLRecordNotFound) {
			e.finish(ctx, target, job)
			return
		}
		e.fail(ctx, target, job, err)
		return
	}
	data := map[string]interface{}{}
	if err := json.Unmarshal(record.Data, &data); err != nil {
		e.fail(ctx, target, job, errors.Wrap(err, "decode record"))
		return
	}

	def, err := Lookup(ctx, field.Shortcut.ID)
	if err != nil {
		e.fail(ctx, target, job, err)
		return
	}
	execCtx, cancel := context.WithTimeout(ctx, executionTimeout(def)+jobTimeoutMargin)
	value, err := e.executor.Execute(execCtx, def, fields, field, data, Env{ProjectUID: project.UID, TableUID: table.UID, FieldUID: field.UID, RecordUID: record.UID})
	cancel()
	if err != nil {
		e.fail(ctx, target, job, err)
		return
	}

	if sameValue(value, data[field.UID]) {
		e.finish(ctx, target, job)
		return
	}
	committed, err := e.hub.CommitServer(ctx, project, table, []collab.Operation{{
		Command: "GenerateCells",
		Actions: []collab.Action{{Action: collab.ActionSetRecord, RecordUID: record.UID, Values: map[string]interface{}{field.UID: value}}},
	}}, func(tx *gorm.DB) (bool, error) {
		// The result is discarded if the cell has been queued again, e.g. the input changed during the execution.
		return db.NewSLShortcutJobsStore(tx).Finish(ctx, job)
	})
	if err != nil {
		log.WithError(err).Error("Failed to write back the shortcut result")
		e.fail(ctx, target, job, err)
		return
	}
	if committed {
		e.broadcast(project.UID, table.UID, []jobState{{fieldUID: job.FieldUID, recordUID: job.RecordUID, status: statusDone}})
	}
}

type jobTarget struct {
	project *db.Project
	table   *db.SLTable
}

func executionTimeout(def *Definition) time.Duration {
	if def.custom != nil && def.custom.TimeoutSeconds > 0 {
		return time.Duration(def.custom.TimeoutSeconds) * time.Second
	}
	return 2 * time.Minute
}

func sameValue(a, b interface{}) bool {
	rawA, errA := json.Marshal(a)
	rawB, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(rawA, rawB)
}

// drop deletes the job of a deleted table or project.
func (e *Engine) drop(ctx context.Context, job *db.SLShortcutJob) {
	if _, err := e.jobs().Finish(ctx, job); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to delete shortcut job")
	}
}

func (e *Engine) finish(ctx context.Context, target *jobTarget, job *db.SLShortcutJob) {
	deleted, err := e.jobs().Finish(ctx, job)
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to delete shortcut job")
		return
	}
	if deleted {
		e.broadcast(target.project.UID, target.table.UID, []jobState{{fieldUID: job.FieldUID, recordUID: job.RecordUID, status: statusDone}})
	}
}

// fail retries the job on transient errors, and marks it as failed otherwise.
func (e *Engine) fail(ctx context.Context, target *jobTarget, job *db.SLShortcutJob, err error) {
	var shortcutErr *Error
	if !errors.As(err, &shortcutErr) {
		if ctx.Err() != nil {
			// The server is shutting down, the job is reset after restarting.
			return
		}
		logrus.WithContext(ctx).WithError(err).WithField("recordUID", job.RecordUID).Error("Failed to execute shortcut job")
		shortcutErr = newError("shortcut::internal_error").transient()
	}

	var updated bool
	status := db.ShortcutJobFailed
	if shortcutErr.Transient && job.Attempts < maxAttempts {
		status = db.ShortcutJobPending
		delay := retryDelays[min(job.Attempts, len(retryDelays))-1]
		updated, err = e.jobs().Retry(ctx, job, dbutil.Now().Add(delay), shortcutErr.JobError())
	} else {
		updated, err = e.jobs().Fail(ctx, job, shortcutErr.JobError())
	}
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to update shortcut job")
		return
	}
	if updated && target != nil {
		e.broadcast(target.project.UID, target.table.UID, []jobState{{fieldUID: job.FieldUID, recordUID: job.RecordUID, status: string(status), err: shortcutErr.JobError()}})
	}
}

// Preview executes the shortcut of the unsaved field on the records without writing back, at most 3 records in parallel.
func (e *Engine) Preview(ctx context.Context, project *db.Project, table *db.SLTable, fields []*db.SLField, field *db.SLField, records []*db.SLRecord, tr Translator) ([]*dto.ShortcutPreview, error) {
	shortcut, err := ValidateField(ctx, fields, field)
	if err != nil {
		return nil, err
	}
	draft := *field
	draft.Shortcut = shortcut
	def, err := Lookup(ctx, shortcut.ID)
	if err != nil {
		return nil, err
	}

	results := make([]*dto.ShortcutPreview, len(records))
	var wg sync.WaitGroup
	for i, record := range records {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := &dto.ShortcutPreview{RecordUID: record.UID}
			results[i] = result
			data := map[string]interface{}{}
			if err := json.Unmarshal(record.Data, &data); err != nil {
				result.Error = tr("shortcut::internal_error")
				return
			}

			execCtx, cancel := context.WithTimeout(ctx, executionTimeout(def))
			defer cancel()
			value, err := e.executor.Execute(execCtx, def, fields, &draft, data, Env{ProjectUID: project.UID, TableUID: table.UID, FieldUID: field.UID, RecordUID: record.UID})
			if err != nil {
				var shortcutErr *Error
				if errors.As(err, &shortcutErr) {
					result.Error = shortcutErr.Text(tr)
				} else {
					logrus.WithContext(ctx).WithError(err).Error("Failed to preview shortcut")
					result.Error = tr("shortcut::internal_error")
				}
				return
			}
			result.Value = value
		}()
	}
	wg.Wait()
	return results, nil
}
