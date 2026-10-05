package shortcut

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

func TestReferencedOptionsSyncAndExecute(t *testing.T) {
	tx := newTestDB(t)
	ctx := context.Background()
	project := &db.Project{Name: "Options"}
	require.NoError(t, tx.Create(project).Error)

	source, err := db.NewSLTablesStore(tx).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Source"})
	require.NoError(t, err)
	target, err := db.NewSLTablesStore(tx).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Target"})
	require.NoError(t, err)

	fieldsStore := db.NewSLFieldsStore(tx)
	create := func(table *db.SLTable, uid string, typ db.SLFieldType, md map[string]interface{}) *db.SLField {
		t.Helper()
		f, err := fieldsStore.Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, UID: uid, Label: uid, Type: typ, Metadata: md})
		require.NoError(t, err)
		return f
	}
	option := func(uid, name string) interface{} {
		return map[string]interface{}{"uid": uid, "name": name, "color": float64(1)}
	}

	parent := create(source, "fldParentS", db.TextFieldType, map[string]interface{}{})
	category := create(source, "fldSourceC", db.SingleSelectFieldType, map[string]interface{}{"options": []interface{}{option("srcA", "Alpha"), option("srcB", "Beta")}})
	local := create(target, "fldParentT", db.TextFieldType, map[string]interface{}{})
	host := create(target, "fldHostXXX", db.SingleSelectFieldType, map[string]interface{}{"options": []interface{}{option("keepA", "Alpha")}, "optionsReference": db.OptionReference{TableUID: source.UID, FieldUID: category.UID, Conditions: []db.OptionCondition{{FieldUID: parent.UID, Operation: db.FilterOperationEqual, ValueFieldUID: local.UID}}}})

	for _, data := range []map[string]interface{}{{parent.UID: "North", category.UID: "srcA"}, {parent.UID: "South", category.UID: "srcB"}} {
		b, _ := json.Marshal(data)
		_, err := db.NewSLRecordsStore(tx).Create(ctx, source.ID, b)
		require.NoError(t, err)
	}

	custom := &db.CustomFieldShortcut{UID: "fscRefernc", Name: "Classify", Enabled: true, ResultType: db.SingleSelectFieldType, TimeoutSeconds: 5,
		Code:      `function execute(p) { return p.categories[0] }`,
		FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{{Key: "categories", Label: "Categories", Component: db.ShortcutFormFieldOptions, Required: true}})}
	require.NoError(t, tx.Create(custom).Error)
	host.Shortcut = &db.FieldShortcut{ID: custom.UID, Inputs: map[string]interface{}{}, AutoUpdate: true}
	require.NoError(t, fieldsStore.SetShortcut(ctx, host.ID, host.Shortcut))
	fields, err := fieldsStore.ListByTableID(ctx, target.ID)
	require.NoError(t, err)

	value, err := Execute(ctx, custom, fields, host, map[string]interface{}{local.UID: "North"}, Env{})
	require.NoError(t, err)
	require.Equal(t, "keepA", value)

	value, err = Execute(ctx, custom, fields, host, map[string]interface{}{}, Env{})
	require.NoError(t, err)
	require.Nil(t, value)

	// Changing a parent clears a previously selected option no longer allowed by the conditions.
	data := map[string]interface{}{local.UID: "South", host.UID: "keepA"}
	changed, err := routeutil.NewRecordValidator(fields).NormalizeReferencedOptions(ctx, tx, data)
	require.NoError(t, err)
	require.True(t, changed)
	require.Nil(t, data[host.UID])

	b, _ := json.Marshal(map[string]interface{}{local.UID: "North", host.UID: "keepA"})
	record, err := db.NewSLRecordsStore(tx).Create(ctx, target.ID, b)
	require.NoError(t, err)
	hub := collab.NewHub(tx)
	engine := NewEngine(tx, hub, 1)

	commit := func(md map[string]interface{}) {
		t.Helper()
		require.NoError(t, hub.Commit(ctx, project, source, nil, "", []collab.Operation{{Command: "EditOptions", Actions: []collab.Action{{Action: collab.ActionSetField, FieldUID: category.UID, Field: &collab.FieldAttrs{Metadata: md}}}}}, func(collab.CommitResult) {}))
	}

	commit(map[string]interface{}{"options": []interface{}{option("srcA", "Renamed"), option("srcB", "Beta"), option("srcC", "New")}})
	require.Eventually(t, func() bool {
		f, err := fieldsStore.GetByID(ctx, host.ID)
		return err == nil && optionNames(f)[0] == "Renamed" && len(optionNames(f)) == 3
	}, 5*time.Second, 20*time.Millisecond)

	fresh, err := fieldsStore.GetByID(ctx, host.ID)
	require.NoError(t, err)
	value, err = Execute(ctx, custom, fields, fresh, map[string]interface{}{local.UID: "North"}, Env{})
	require.NoError(t, err)
	require.Equal(t, "keepA", value)

	stored, err := db.NewSLRecordsStore(tx).GetByID(ctx, record.ID)
	require.NoError(t, err)
	var storedData map[string]interface{}
	require.NoError(t, json.Unmarshal(stored.Data, &storedData))
	require.Equal(t, "keepA", storedData[host.UID])

	jobs := db.NewSLShortcutJobsStore(tx)
	require.Eventually(t, func() bool {
		list, _ := jobs.ListByTableID(ctx, target.ID)
		return len(list) > 0
	}, 5*time.Second, 20*time.Millisecond)
	job, err := jobs.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, job)
	engine.process(ctx, job)

	// Color-only synchronization keeps identity and does not regenerate the shortcut.
	colorMD := map[string]interface{}{"options": []interface{}{option("srcA", "Renamed"), option("srcB", "Beta"), option("srcC", "New")}}
	colorMD["options"].([]interface{})[0].(map[string]interface{})["color"] = float64(8)
	commit(colorMD)
	require.Eventually(t, func() bool {
		f, err := fieldsStore.GetByID(ctx, host.ID)
		if err != nil {
			return false
		}

		md := f.Metadata.Data().(map[string]interface{})
		return md["options"].([]interface{})[0].(map[string]interface{})["color"] == float64(8)
	}, 5*time.Second, 20*time.Millisecond)

	list, err := jobs.ListByTableID(ctx, target.ID)
	require.NoError(t, err)
	require.Empty(t, list)
}
