package dto

import (
	"encoding/json"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow"
)

// WorkflowListItem is a workflow in the list.
type WorkflowListItem struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// TriggerType is the trigger type of the draft, PublishedTriggerType is the trigger type of the published revision, empty if unpublished.
	TriggerType          string `json:"triggerType" enums:",api,record_created,record_updated,record_matched,record_deleted,date_reached,schedule"`
	PublishedTriggerType string `json:"publishedTriggerType"`
	// Method and Path are the entry of the API trigger in the draft, empty for other triggers.
	Method string `json:"method"`
	Path   string `json:"path"`
	// PublishedMethod and PublishedPath are the API entry being served, empty if unpublished or for other triggers.
	PublishedMethod  string     `json:"publishedMethod"`
	PublishedPath    string     `json:"publishedPath"`
	Enabled          bool       `json:"enabled"`
	PublishedVersion int        `json:"publishedVersion"`
	PublishedAt      *time.Time `json:"publishedAt"`
	// HasUnpublishedChanges indicates the draft has been modified since the latest publishing.
	HasUnpublishedChanges bool      `json:"hasUnpublishedChanges"`
	StepCount             int       `json:"stepCount"`
	AsyncCount            int       `json:"asyncCount"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
} // @name WorkflowListItem

// ToWorkflowListItem converts the list item, the trigger type is empty if the draft cannot be parsed.
func ToWorkflowListItem(w *db.Workflow) *WorkflowListItem {
	item := &WorkflowListItem{
		UID: w.UID, Name: w.Name, Description: w.Description, PublishedTriggerType: w.TriggerType,
		PublishedMethod: w.Method, PublishedPath: w.Path, Enabled: w.Enabled,
		PublishedVersion: w.PublishedVersion, PublishedAt: w.PublishedAt,
		HasUnpublishedChanges: w.PublishedRevisionID == 0 || w.PublishedDraftVersion != w.DraftVersion,
		CreatedAt:             w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
	if spec, err := workflow.ParseSpec(w.Draft); err == nil {
		item.TriggerType = spec.Trigger.Type
		if spec.IsAPI() {
			item.Method, item.Path = spec.Trigger.API.Method, spec.Trigger.API.Path
		}
		spec.AllSteps(func(*workflow.Step) { item.StepCount++ })
		item.AsyncCount = len(spec.Async)
	}
	return item
}

// Workflow is the workflow in the editor.
type Workflow struct {
	WorkflowListItem
	Draft        *workflow.Spec `json:"draft"`
	DraftVersion int64          `json:"draftVersion"`
	// Issues are the issues in the draft that must be fixed before publishing.
	Issues []workflow.Issue `json:"issues"`
} // @name Workflow

// WorkflowRevision is a published revision.
type WorkflowRevision struct {
	Version     int        `json:"version"`
	TriggerType string     `json:"triggerType"`
	Method      string     `json:"method"`
	Path        string     `json:"path"`
	CreatedAt   time.Time  `json:"createdAt"`
	CreatedBy   *UserBrief `json:"createdBy"`
} // @name WorkflowRevision

// SaveWorkflowDraftResp is the result of saving a draft.
type SaveWorkflowDraftResp struct {
	DraftVersion int64            `json:"draftVersion"`
	Issues       []workflow.Issue `json:"issues"`
	UpdatedAt    time.Time        `json:"updatedAt"`
} // @name SaveWorkflowDraftResp

// PublishWorkflowResp is the result of publishing, the workflow is not published if Issues is not empty.
type PublishWorkflowResp struct {
	Issues   []workflow.Issue `json:"issues"`
	Workflow *Workflow        `json:"workflow,omitempty"`
} // @name PublishWorkflowResp

// DebugWorkflowResp is the result of debugging.
type DebugWorkflowResp struct {
	workflow.RunResult
	Issues []workflow.Issue `json:"issues"`
} // @name DebugWorkflowResp

// WorkflowJob is a background job of a run: an async flow triggered by API, or the main flow of other triggers (flowID is main).
type WorkflowJob struct {
	ID     int64  `json:"id"`
	FlowID string `json:"flowID"`
	// Event is the triggering event of the async flow, request_received or succeeded, or the trigger type for the main flow.
	Event      string               `json:"event"`
	Status     string               `json:"status" enums:"pending,running,succeeded,failed"`
	Attempts   int                  `json:"attempts"`
	Error      string               `json:"error"`
	Uncertain  bool                 `json:"uncertain"`
	Trace      []workflow.TraceStep `json:"trace"`
	CreatedAt  time.Time            `json:"createdAt"`
	FinishedAt *time.Time           `json:"finishedAt"`
} // @name WorkflowJob

func ToWorkflowJob(job *db.WorkflowJob) *WorkflowJob {
	trace := []workflow.TraceStep{}
	_ = json.Unmarshal(job.Trace, &trace)
	return &WorkflowJob{
		ID: job.ID, FlowID: job.FlowID, Event: job.Event, Status: string(job.Status), Attempts: job.Attempts,
		Error: job.Error, Uncertain: job.Uncertain, Trace: trace, CreatedAt: job.CreatedAt, FinishedAt: job.FinishedAt,
	}
}

// WorkflowRun is the run of a trigger.
type WorkflowRun struct {
	UID     string `json:"uid"`
	Version int    `json:"version"`
	// Trigger is the source of the run: http, debug, or the trigger type of a background trigger.
	Trigger    string         `json:"trigger"`
	Status     string         `json:"status" enums:"queued,running,succeeded,failed,rejected"`
	StatusCode int            `json:"statusCode"`
	DurationMs int64          `json:"durationMs"`
	IP         string         `json:"ip"`
	CreatedAt  time.Time      `json:"createdAt"`
	Jobs       []*WorkflowJob `json:"jobs"`
	// Request, Event, Response, Trace and Error are only returned in the detail, Event is the trigger data of a background trigger.
	Request  json.RawMessage      `json:"request,omitempty" swaggertype:"object"`
	Event    json.RawMessage      `json:"event,omitempty" swaggertype:"object"`
	Response json.RawMessage      `json:"response,omitempty" swaggertype:"object"`
	Trace    []workflow.TraceStep `json:"trace,omitempty"`
	Error    json.RawMessage      `json:"error,omitempty" swaggertype:"object"`
} // @name WorkflowRun

func ToWorkflowRun(run *db.WorkflowRun, jobs []*db.WorkflowJob, detail bool) *WorkflowRun {
	out := &WorkflowRun{
		UID: run.UID, Version: run.Version, Trigger: string(run.Trigger), Status: string(run.Status), StatusCode: run.StatusCode,
		DurationMs: run.DurationMs, IP: run.IP, CreatedAt: run.CreatedAt, Jobs: []*WorkflowJob{},
	}
	for _, job := range jobs {
		if job.RunID == run.ID {
			j := ToWorkflowJob(job)
			if !detail {
				j.Trace = nil
			}
			out.Jobs = append(out.Jobs, j)
		}
	}
	if detail {
		out.Request, out.Event = json.RawMessage(run.Request), json.RawMessage(run.Event)
		out.Response, out.Error = json.RawMessage(run.Response), json.RawMessage(run.Error)
		out.Trace = []workflow.TraceStep{}
		_ = json.Unmarshal(run.Trace, &out.Trace)
	}
	return out
}

// WorkflowKey is a credential for calling an API workflow, the plaintext is only returned on creation.
type WorkflowKey struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
} // @name WorkflowKey

func ToWorkflowKey(key *db.WorkflowKey) *WorkflowKey {
	return &WorkflowKey{ID: key.ID, Name: key.Name, Prefix: key.Prefix, LastUsedAt: key.LastUsedAt, CreatedAt: key.CreatedAt}
}

// CreateWorkflowKeyResp returns the new key and its plaintext.
type CreateWorkflowKeyResp struct {
	Key    *WorkflowKey `json:"key"`
	Secret string       `json:"secret"`
} // @name CreateWorkflowKeyResp

// WorkflowModules are the DSL definitions of the step modules and credential types, which the editor renders nodes and forms with.
type WorkflowModules struct {
	Modules         []*workflow.ModuleDef         `json:"modules"`
	CredentialTypes []*workflow.CredentialTypeDef `json:"credentialTypes"`
	// SecretsReady is false if auth.secret_key is not configured, in which case secret fields can not be saved.
	SecretsReady bool `json:"secretsReady"`
} // @name WorkflowModules

// Credential is a project credential without the values of its secret fields.
type Credential struct {
	UID  string                 `json:"uid"`
	Type string                 `json:"type"`
	Name string                 `json:"name"`
	Data map[string]interface{} `json:"data"`
	// Secrets lists the saved secret fields.
	Secrets   []string  `json:"secrets"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
} // @name Credential

// ToCredential converts the credential, secrets are the names of the saved secret fields.
func ToCredential(c *db.Credential, secrets []string) *Credential {
	data := map[string]interface{}{}
	_ = json.Unmarshal(c.Data, &data)
	if secrets == nil {
		secrets = []string{}
	}
	return &Credential{UID: c.UID, Type: c.Type, Name: c.Name, Data: data, Secrets: secrets, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}
