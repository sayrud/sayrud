package collab

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// Batch is a transaction that writes records across multiple tables, created by Hub.Batch.
type Batch struct {
	ctx    context.Context
	tx     *gorm.DB
	tables map[int64]*batchTable
}

type batchTable struct {
	table  *db.SLTable
	fields []*db.SLField
	dirty  DirtyScope
	change Change
	events []RecordEvent
}

// BatchOptions are the options of Hub.Batch.
type BatchOptions struct {
	// DryRun rolls back the transaction after fn succeeds, without committing or broadcasting.
	DryRun bool
	// Origin is the source of the writes, by which the record changes trigger workflows.
	Origin Origin
}

var errDryRun = errors.New("dry run")

// Batch runs fn in a single transaction: it first locks tables in ascending ID order, keeping a consistent lock order with other batches and serializing with WebSocket, REST and background writes;
// the record writes in fn append the changeset of each table before committing, which are broadcast and notified to the field shortcuts together after committing.
func (h *Hub) Batch(ctx context.Context, project *db.Project, tables []*db.SLTable, opts BatchOptions, fn func(b *Batch) error) error {
	tables = lo.UniqBy(tables, func(t *db.SLTable) int64 { return t.ID })
	sort.Slice(tables, func(i, j int) bool { return tables[i].ID < tables[j].ID })

	var changesets []*Changeset
	var touched []*batchTable
	queued := false
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		b := &Batch{ctx: ctx, tx: tx, tables: make(map[int64]*batchTable, len(tables))}
		for _, table := range tables {
			if err := dbutil.LockTable(ctx, tx, table.ID); err != nil {
				return errors.Wrapf(err, "lock table %d", table.ID)
			}
			b.tables[table.ID] = &batchTable{table: table}
		}

		if err := fn(b); err != nil {
			return err
		}
		if opts.DryRun {
			return errDryRun
		}

		for _, table := range tables {
			bt := b.tables[table.ID]
			q, err := h.recordsChanged(ctx, tx, project, table, opts.Origin, bt.events)
			if err != nil {
				return err
			}
			queued = queued || q
			if bt.dirty.IsEmpty() {
				continue
			}
			dirty := bt.dirty
			changeset, err := appendChangeset(ctx, tx, table, "", "", []Operation{{
				Command: "Refresh", Actions: []Action{{Action: ActionDirty, Dirty: &dirty}},
			}})
			if err != nil {
				return err
			}
			changesets = append(changesets, changeset)
			touched = append(touched, bt)
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		return nil
	}
	if err != nil {
		return err
	}

	for i, changeset := range changesets {
		bt := touched[i]
		h.broadcastChangeset(project, changeset, nil)
		h.NotifyChange(ctx, project, bt.table, &bt.change)
		h.notifyOptionReferences(ctx, project, bt.table, changeset.Operations)
	}
	h.runsQueued(queued)
	return nil
}

// Tx returns the transaction of the batch, e.g. to save the caller's results atomically with the records.
func (b *Batch) Tx() *gorm.DB {
	return b.tx
}

var ErrBatchTableNotLocked = errors.New("the table is not locked by the batch")

func (b *Batch) table(table *db.SLTable) (*batchTable, error) {
	bt, ok := b.tables[table.ID]
	if !ok {
		return nil, ErrBatchTableNotLocked
	}
	if bt.fields == nil {
		fields, err := db.NewSLFieldsStore(b.tx).ListByTableID(b.ctx, table.ID)
		if err != nil {
			return nil, errors.Wrap(err, "list fields")
		}
		bt.fields = fields
	}
	return bt, nil
}

// Fields returns the current fields of the locked table.
func (b *Batch) Fields(table *db.SLTable) ([]*db.SLField, error) {
	bt, err := b.table(table)
	if err != nil {
		return nil, err
	}
	return bt.fields, nil
}

// CreateRecord validates the values keyed by field UID with the same rules as the REST API and creates the record.
func (b *Batch) CreateRecord(table *db.SLTable, data map[string]interface{}) (*db.SLRecord, error) {
	bt, err := b.table(table)
	if err != nil {
		return nil, err
	}
	raw, err := routeutil.Validate(b.ctx, b.tx, table.ID, data)
	if err != nil {
		return nil, err
	}
	record, err := db.NewSLRecordsStore(b.tx).Create(b.ctx, table.ID, raw)
	if err != nil {
		return nil, errors.Wrap(err, "create record")
	}

	bt.dirty.addRecords(record.UID)
	bt.change.AddRecord(record.UID, keysOf(raw))
	bt.events = append(bt.events, RecordEvent{Kind: RecordCreated, Record: record})
	return record, nil
}

// UpdateRecord re-reads the record in the transaction and replaces its values, it returns db.ErrSLRecordNotFound if the record does not belong to the table.
func (b *Batch) UpdateRecord(table *db.SLTable, recordUID string, update func(current map[string]interface{}) map[string]interface{}) (*db.SLRecord, error) {
	bt, err := b.table(table)
	if err != nil {
		return nil, err
	}
	store := db.NewSLRecordsStore(b.tx)
	record, err := store.GetByUID(b.ctx, recordUID)
	if err != nil {
		return nil, err
	}
	if record.SLTableID != table.ID {
		return nil, db.ErrSLRecordNotFound
	}

	current := map[string]interface{}{}
	_ = json.Unmarshal(record.Data, &current)
	raw, err := routeutil.Validate(b.ctx, b.tx, table.ID, update(current))
	if err != nil {
		return nil, err
	}
	changed := changedKeys(record.Data, raw)
	if err := store.Update(b.ctx, record.ID, raw); err != nil {
		return nil, errors.Wrap(err, "update record")
	}
	previous := record.Data
	record.Data, record.UpdatedAt = datatypes.JSON(raw), time.Now()

	bt.dirty.addRecords(record.UID)
	bt.change.AddRecord(record.UID, changed)
	bt.events = append(bt.events, RecordEvent{Kind: RecordUpdated, Record: record, Previous: previous, Fields: changed})
	return record, nil
}

func keysOf(raw []byte) []string {
	data := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &data)
	return lo.Keys(data)
}

// changedKeys returns the field UIDs whose values differ, compared by JSON semantics, ignoring formatting and the order of object properties.
func changedKeys(before, after []byte) []string {
	a, b := map[string]interface{}{}, map[string]interface{}{}
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(after, &b)
	keys := []string{}
	for _, k := range lo.Union(lo.Keys(a), lo.Keys(b)) {
		if !reflect.DeepEqual(a[k], b[k]) {
			keys = append(keys, k)
		}
	}
	return keys
}
