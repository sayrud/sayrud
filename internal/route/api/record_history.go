package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

const (
	// changeMergeWindow is the longest time span of the consecutive updates of a writer merged into one history entry.
	changeMergeWindow = 5 * time.Minute
	// changeScanBatch is the number of changes read at a time when merging the history.
	changeScanBatch = 200
	// timelineSummaryLength is the number of characters of a timeline summary.
	timelineSummaryLength = 500
	// timelineValueLimit is the largest JSON size of a value inlined in a timeline change.
	timelineValueLimit = 1 << 10
)

// changeGroup is the history entry merged from the consecutive changes, read from the newest.
type changeGroup struct {
	// entry is the merged history entry.
	entry *dto.RecordChange
	// fields are the changed fields of the entry keyed by field UID.
	fields map[string]*dto.RecordFieldChange
	// newest is the time of the newest change merged.
	newest time.Time
	// oldestID is the smallest ID of the changes merged.
	oldestID int64
}

// newChangeGroup returns the group starting from the newest change.
func newChangeGroup(c *db.SLRecordChange) *changeGroup {
	g := &changeGroup{
		entry:  dto.NewRecordChange(c),
		fields: map[string]*dto.RecordFieldChange{},
		newest: c.CreatedAt,
	}
	g.add(c)
	return g
}

// accepts reports whether the older change continues the updates of the same writer within the merge window.
func (g *changeGroup) accepts(c *db.SLRecordChange) bool {
	return g.entry.Kind == db.SLRecordChangeUpdated && c.Kind == db.SLRecordChangeUpdated &&
		lo.FromPtr(g.entry.Actor) == c.Writer() && g.newest.Sub(c.CreatedAt) <= changeMergeWindow
}

// add merges the older change: a field keeps the newest value and label, and the oldest previous value.
func (g *changeGroup) add(c *db.SLRecordChange) {
	g.oldestID = c.ID
	if c.Kind != db.SLRecordChangeUpdated {
		return
	}
	if f, ok := g.fields[c.FieldUID]; ok {
		f.OldValue = json.RawMessage(c.OldValue)
		return
	}
	f := dto.NewRecordFieldChange(c)
	g.fields[c.FieldUID] = f
	g.entry.Fields = append(g.entry.Fields, f)
}

// finish drops the fields changed back to their previous values and orders the fields as the table, it returns nil if nothing is left.
func (g *changeGroup) finish(positions map[string]int) *dto.RecordChange {
	if g.entry.Kind != db.SLRecordChangeUpdated {
		return g.entry
	}
	fields := lo.Filter(g.entry.Fields, func(f *dto.RecordFieldChange, _ int) bool { return !sameJSON(f.OldValue, f.NewValue) })
	if len(fields) == 0 {
		return nil
	}
	sort.SliceStable(fields, func(i, j int) bool {
		pi, iok := positions[fields[i].FieldUID]
		pj, jok := positions[fields[j].FieldUID]
		if iok != jok {
			return iok
		}
		return pi < pj
	})
	g.entry.Fields = fields
	return g.entry
}

// sameJSON compares the values by JSON semantics, the absent values are equal.
func sameJSON(a, b json.RawMessage) bool {
	var x, y interface{}
	if len(a) > 0 {
		_ = json.Unmarshal(a, &x)
	}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &y)
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return bytes.Equal(ja, jb)
}

// ListChanges
// @Summary List the history of a record
// @Tags Record history
// @Description Return the creation, the deletion and the field changes of the record the newest first, with the field labels when written.
// @Description The consecutive updates of the same writer within 5 minutes are merged into one entry, keeping the earliest previous value and the latest value of each field;
// @Description the fields changed back are dropped. The writes of the server, e.g. field shortcuts and type conversions, are not recorded, and the system fields never appear.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param cursor query int false "Return the history older than the cursor"
// @Param limit query int false "Maximum number of entries, defaults to 20 and at most 100"
// @Success 200 {object} dto.ListRecordChangesResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID listRecordChanges
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/changes [get]
func (schemalessRoute) ListChanges(ctx context.Context, table *db.SLTable, record *db.SLRecord) error {
	c := ctx.Request().Context()
	fields, err := db.SLFields.ListByTableID(c, table.ID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list fields")
		return ctx.ApiServerError()
	}
	positions := map[string]int{}
	system := map[string]bool{}
	for i, f := range fields {
		positions[f.UID] = i
		system[f.UID] = f.Type.IsSystem()
	}

	limit := pageLimit(ctx)
	resp := dto.ListRecordChangesResp{Changes: []*dto.RecordChange{}}
	var group *changeGroup
	before := int64(ctx.QueryInt64("cursor"))
scan:
	for {
		rows, err := db.SLRecordChanges.ListByRecord(c, record.ID, before, changeScanBatch)
		if err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to list record changes")
			return ctx.ApiServerError()
		}
		for _, row := range rows {
			before = row.ID
			if row.Kind == db.SLRecordChangeUpdated && system[row.FieldUID] {
				continue
			}
			if group != nil && group.accepts(row) {
				group.add(row)
				continue
			}
			if group != nil {
				if entry := group.finish(positions); entry != nil {
					resp.Changes = append(resp.Changes, entry)
				}
				if len(resp.Changes) == limit {
					// The page ends at a group boundary, so the next page starts a new group.
					resp.NextCursor = group.oldestID
					group = nil
					break scan
				}
			}
			group = newChangeGroup(row)
		}
		if len(rows) < changeScanBatch {
			break
		}
	}
	if group != nil {
		if entry := group.finish(positions); entry != nil {
			resp.Changes = append(resp.Changes, entry)
		}
	}
	return ctx.ApiSuccess(resp)
}

// GetChange
// @Summary Get a change of a record
// @Tags Record history
// @Description Return a single change without merging, e.g. a change truncated in the timeline.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param changeID path int true "Change ID"
// @Success 200 {object} dto.RecordChange
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table, record or change not found"
// @Failure 500 {string} string "Internal server error"
// @ID getRecordChange
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/changes/{changeID} [get]
func (schemalessRoute) GetChange(ctx context.Context, record *db.SLRecord) error {
	id, _ := strconv.ParseInt(ctx.Param("changeID"), 10, 64)
	change, err := db.SLRecordChanges.GetByID(ctx.Request().Context(), record.ID, id)
	if err != nil {
		if errors.Is(err, db.ErrSLRecordChangeNotFound) {
			return ctx.ApiError(http.StatusNotFound, "record::change_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get record change")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToRecordChange(change))
}

// encodeTimelineCursor returns the opaque cursor of the position.
func encodeTimelineCursor(c db.TimelineCursor) string {
	raw := strconv.FormatInt(c.Time.UnixMicro(), 10) + "." + strconv.Itoa(c.Rank) + "." + strconv.FormatInt(c.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeTimelineCursor returns the position of the cursor, nil for an empty cursor.
func decodeTimelineCursor(s string) (*db.TimelineCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "decode")
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed cursor")
	}
	micro, err1 := strconv.ParseInt(parts[0], 10, 64)
	rank, err2 := strconv.Atoi(parts[1])
	id, err3 := strconv.ParseInt(parts[2], 10, 64)
	if err := errors.CombineErrors(err1, errors.CombineErrors(err2, err3)); err != nil {
		return nil, err
	}
	cursor := db.NewTimelineCursor(time.UnixMicro(micro), rank, id)
	return &cursor, nil
}

// timelineEntry is an item of the timeline with its position.
type timelineEntry struct {
	cursor  db.TimelineCursor
	comment *db.SLRecordComment
	change  *db.SLRecordChange
}

// before reports whether e comes before o in the order.
func (e timelineEntry) before(o timelineEntry, asc bool) bool {
	a, b := e.cursor, o.cursor
	less := a.Time.Before(b.Time) || a.Time.Equal(b.Time) && (a.Rank < b.Rank || a.Rank == b.Rank && a.ID < b.ID)
	return less == asc
}

// truncateText returns the first n characters of the text and whether it is shortened.
func truncateText(s string, n int) (string, bool) {
	runes := []rune(s)
	if len(runes) <= n {
		return s, false
	}
	return string(runes[:n]), true
}

// Timeline
// @Summary Read the timeline of a record
// @Tags Record history
// @Description Return the live comments and the unmerged field changes of the record in one timeline ordered by time, for agents to read the context of the record.
// @Description Every item has a stable ID, the writer and a text summary; long summaries and values are truncated, and ref is the API path reading the full comment or change.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param cursor query string false "Return the items after the cursor returned by the previous page"
// @Param limit query int false "Maximum number of items, defaults to 20 and at most 100"
// @Param order query string false "desc for the newest first (default), asc for the oldest first" Enums(desc, asc)
// @Success 200 {object} dto.RecordTimelineResp
// @Failure 400 {string} string "Invalid cursor"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID getRecordTimeline
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/timeline [get]
func (schemalessRoute) Timeline(ctx context.Context, project *db.Project, table *db.SLTable, record *db.SLRecord) error {
	c := ctx.Request().Context()
	cursor, err := decodeTimelineCursor(ctx.Query("cursor"))
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "record::invalid_cursor")
	}
	asc := ctx.Query("order") == "asc"
	limit := pageLimit(ctx)

	comments, err := db.SLRecordComments.ListTimeline(c, record.ID, cursor, asc, limit+1)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list timeline comments")
		return ctx.ApiServerError()
	}
	changes, err := db.SLRecordChanges.ListTimeline(c, record.ID, cursor, asc, limit+1)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list timeline changes")
		return ctx.ApiServerError()
	}
	entries := make([]timelineEntry, 0, len(comments)+len(changes))
	for _, cm := range comments {
		entries = append(entries, timelineEntry{
			cursor:  cm.TimelineCursor(),
			comment: cm,
		})
	}
	for _, ch := range changes {
		entries = append(entries, timelineEntry{
			cursor: ch.TimelineCursor(),
			change: ch,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].before(entries[j], asc) })

	resp := dto.RecordTimelineResp{Items: []*dto.TimelineItem{}}
	if len(entries) > limit {
		entries = entries[:limit]
		resp.NextCursor = encodeTimelineCursor(entries[limit-1].cursor)
	}

	fields, err := db.SLFields.ListByTableID(c, table.ID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list fields")
		return ctx.ApiServerError()
	}
	byUID := lo.KeyBy(fields, func(f *db.SLField) string { return f.UID })
	var memberIDs []int64
	for _, e := range entries {
		if e.change != nil && e.change.FieldType == db.MemberFieldType {
			for _, raw := range [][]byte{e.change.OldValue, e.change.NewValue} {
				var value interface{}
				_ = json.Unmarshal(raw, &value)
				memberIDs = append(memberIDs, routeutil.MemberIDs(value)...)
			}
		}
	}
	var users []*db.User
	if memberIDs = lo.Uniq(memberIDs); len(memberIDs) > 0 {
		if users, err = db.Users.ListByIDs(c, memberIDs); err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to list member users")
			return ctx.ApiServerError()
		}
	}
	directory := routeutil.NewDirectory(users, nil, nil)
	boolText := func(v bool) string { return ctx.Tr(lo.Ternary(v, "common::yes", "common::no")) }

	base := fmt.Sprintf("/projects/%s/tables/%s/records/%s", project.UID, table.UID, record.UID)
	for _, e := range entries {
		if cm := e.comment; cm != nil {
			summary, truncated := truncateText(cm.Body, timelineSummaryLength)
			ref := base + "/comments/" + strconv.FormatInt(cm.ID, 10)
			resp.Items = append(resp.Items, dto.NewCommentTimelineItem(ref, cm, summary, truncated))
			continue
		}
		ch := e.change
		var summary string
		withValues := true
		switch ch.Kind {
		case db.SLRecordChangeCreated:
			summary = ctx.Tr("record::timeline_created")
		case db.SLRecordChangeDeleted:
			summary = ctx.Tr("record::timeline_deleted")
		default:
			field := &db.SLField{
				UID:   ch.FieldUID,
				Label: ch.FieldLabel,
				Type:  ch.FieldType,
			}
			if current, ok := byUID[ch.FieldUID]; ok && current.Type == ch.FieldType {
				field = current
			}
			summary = fmt.Sprintf("%s: %s → %s", ch.FieldLabel, changeValueText(field, ch.OldValue, boolText, directory), changeValueText(field, ch.NewValue, boolText, directory))
			withValues = len(ch.OldValue) <= timelineValueLimit && len(ch.NewValue) <= timelineValueLimit
		}
		summary, cut := truncateText(summary, timelineSummaryLength)
		ref := base + "/changes/" + strconv.FormatInt(ch.ID, 10)
		resp.Items = append(resp.Items, dto.NewChangeTimelineItem(ref, ch, summary, cut || !withValues, withValues))
	}
	return ctx.ApiSuccess(resp)
}

// changeValueText returns the display text of a changed value, the raw JSON if it has no text, e.g. the options of a deleted field.
func changeValueText(field *db.SLField, raw []byte, boolText func(bool) string, directory *routeutil.Directory) string {
	if len(raw) == 0 {
		return "∅"
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	if text := collab.ValueText(field, value, boolText, directory); text != "" {
		return text
	}
	return string(raw)
}
