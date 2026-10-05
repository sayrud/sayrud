package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptionReference(t *testing.T) {
	tx := newTestDB(t, &Project{}, &SLTable{}, &SLField{}, &SLRecord{})
	ctx := context.Background()
	project := &Project{Name: "References"}
	require.NoError(t, tx.Create(project).Error)

	tables, fields := NewSLTablesStore(tx), NewSLFieldsStore(tx)
	source, err := tables.Create(ctx, project.ID, CreateSLTableOptions{Name: "Dictionary"})
	require.NoError(t, err)
	target, err := tables.Create(ctx, project.ID, CreateSLTableOptions{Name: "Records"})
	require.NoError(t, err)

	option := func(uid, name string) interface{} {
		return map[string]interface{}{"uid": uid, "name": name, "color": float64(1)}
	}
	create := func(table *SLTable, uid string, typ SLFieldType, md map[string]interface{}) *SLField {
		t.Helper()
		f, err := fields.Create(ctx, CreateSLFieldOptions{SLTableID: table.ID, UID: uid, Label: uid, Type: typ, Metadata: md})
		require.NoError(t, err)
		return f
	}

	parent := create(source, "fldSourceP", SingleSelectFieldType, map[string]interface{}{"options": []interface{}{option("srcNorth", "North"), option("srcSouth", "South")}})
	category := create(source, "fldSourceC", MultiSelectFieldType, map[string]interface{}{"options": []interface{}{option("srcA", "Alpha"), option("srcB", "Beta"), option("srcC", "Gamma"), option("srcD", "Unused")}})
	flag := create(source, "fldActiveX", CheckboxFieldType, map[string]interface{}{})
	local := create(target, "fldParentX", SingleSelectFieldType, map[string]interface{}{"options": []interface{}{option("ownNorth", "North"), option("ownSouth", "South")}})
	alternate := create(target, "fldParentY", SingleSelectFieldType, map[string]interface{}{"options": []interface{}{option("altNorth", "North"), option("altSouth", "South")}})

	for _, data := range []map[string]interface{}{
		{parent.UID: "srcNorth", category.UID: []string{"srcA", "srcB"}, flag.UID: true},
		{parent.UID: "srcSouth", category.UID: []string{"srcC"}, flag.UID: true},
		{parent.UID: "srcNorth", category.UID: []string{"srcC"}},
	} {
		b, err := json.Marshal(data)
		require.NoError(t, err)
		_, err = NewSLRecordsStore(tx).Create(ctx, source.ID, b)
		require.NoError(t, err)
	}

	md := map[string]interface{}{"options": []interface{}{option("keepA", "Alpha")}, "default": "keepA", "optionsReference": OptionReference{TableUID: source.UID, FieldUID: category.UID, Conditions: []OptionCondition{
		{FieldUID: parent.UID, Operation: FilterOperationEqual, ValueFieldUID: local.UID},
		{FieldUID: flag.UID, Operation: FilterOperationEqual, Value: "true"},
	}}}
	host := create(target, "fldHostXXX", SingleSelectFieldType, md)
	require.Equal(t, "", md["default"])
	require.Len(t, selectOptions(md), 4) // Includes unused options until conditions are applied.
	require.Equal(t, "keepA", selectOptions(md)[0].(map[string]interface{})["uid"])

	resolve := func(data map[string]interface{}) []interface{} {
		t.Helper()
		f, err := FilterReferencedOptions(ctx, tx, host, []*SLField{local, alternate, host}, data)
		require.NoError(t, err)
		return selectOptions(f.Metadata.Data().(map[string]interface{}))
	}

	require.Len(t, resolve(map[string]interface{}{local.UID: "ownNorth"}), 2)
	require.Equal(t, "Gamma", resolve(map[string]interface{}{local.UID: "ownSouth"})[0].(map[string]interface{})["name"])
	require.Empty(t, resolve(map[string]interface{}{}))

	// Any-condition matching combines fields while keeping missing values and other tables excluded.
	foreignData, err := json.Marshal(map[string]interface{}{parent.UID: "srcSouth", category.UID: []string{"srcD"}})
	require.NoError(t, err)
	_, err = NewSLRecordsStore(tx).Create(ctx, target.ID, foreignData)
	require.NoError(t, err)
	deleted, err := NewSLRecordsStore(tx).Create(ctx, source.ID, foreignData)
	require.NoError(t, err)
	require.NoError(t, tx.Delete(deleted).Error)

	ref := OptionReference{TableUID: source.UID, FieldUID: category.UID, Match: "all", Conditions: []OptionCondition{
		{FieldUID: parent.UID, Operation: FilterOperationEqual, ValueFieldUID: local.UID},
		{FieldUID: parent.UID, Operation: FilterOperationEqual, ValueFieldUID: alternate.UID},
	}}
	md["optionsReference"] = ref
	require.NoError(t, fields.SetMetadata(ctx, host.ID, md))
	host, err = fields.GetByID(ctx, host.ID)
	require.NoError(t, err)
	require.Empty(t, resolve(map[string]interface{}{local.UID: "ownNorth", alternate.UID: "altSouth"}))

	ref.Match = "any"
	md["optionsReference"] = ref
	require.NoError(t, fields.SetMetadata(ctx, host.ID, md))
	host, err = fields.GetByID(ctx, host.ID)
	require.NoError(t, err)
	require.Len(t, resolve(map[string]interface{}{local.UID: "ownNorth", alternate.UID: "altSouth"}), 3)
	require.Equal(t, "Gamma", resolve(map[string]interface{}{local.UID: "ownSouth"})[0].(map[string]interface{})["name"])
	require.Empty(t, resolve(nil))

	// Fixed and empty conditions use the same SQL filter implementation.
	md["optionsReference"] = OptionReference{TableUID: source.UID, FieldUID: category.UID, Conditions: []OptionCondition{{FieldUID: flag.UID, Operation: FilterOperationEmpty}}}
	require.NoError(t, fields.SetMetadata(ctx, host.ID, md))
	host, err = fields.GetByID(ctx, host.ID)
	require.NoError(t, err)
	require.Equal(t, "Gamma", resolve(nil)[0].(map[string]interface{})["name"])

	md["optionsReference"] = OptionReference{TableUID: source.UID, FieldUID: category.UID}
	require.NoError(t, fields.SetMetadata(ctx, host.ID, md))
	host, err = fields.GetByID(ctx, host.ID)
	require.NoError(t, err)
	require.Len(t, resolve(nil), 4)

	// Renaming follows source identity, preserving the existing selected ID.
	sourceMD := category.Metadata.Data().(map[string]interface{})
	selectOptions(sourceMD)[0].(map[string]interface{})["name"] = "Renamed"
	require.NoError(t, fields.SetMetadata(ctx, category.ID, sourceMD))
	require.NoError(t, fields.SetMetadata(ctx, host.ID, md))
	require.Equal(t, "keepA", selectOptions(md)[0].(map[string]interface{})["uid"])
	require.Equal(t, "Renamed", selectOptions(md)[0].(map[string]interface{})["name"])

	// Source -> host -> source is rejected, as are references outside the project.
	sourceMD["optionsReference"] = OptionReference{TableUID: target.UID, FieldUID: host.UID}
	require.ErrorIs(t, fields.SetMetadata(ctx, category.ID, sourceMD), ErrOptionReference)

	other := &Project{Name: "Private"}
	require.NoError(t, tx.Create(other).Error)
	foreign, err := tables.Create(ctx, other.ID, CreateSLTableOptions{Name: "Private"})
	require.NoError(t, err)
	foreignField := create(foreign, "fldPrivate", SingleSelectFieldType, map[string]interface{}{})
	md["optionsReference"] = OptionReference{TableUID: foreign.UID, FieldUID: foreignField.UID}
	require.ErrorIs(t, fields.SetMetadata(ctx, host.ID, md), ErrOptionReference)

	md["optionsReference"] = OptionReference{TableUID: source.UID, FieldUID: flag.UID}
	require.ErrorIs(t, fields.SetMetadata(ctx, host.ID, md), ErrOptionReference)
}

func TestOptionReferenceMatch(t *testing.T) {
	for _, mode := range []string{"", "all", "any"} {
		ref, err := ReadOptionReference(map[string]interface{}{"optionsReference": OptionReference{TableUID: "table", FieldUID: "field", Match: mode}})
		require.NoError(t, err)
		require.Equal(t, mode, ref.Match)
	}

	_, err := ReadOptionReference(map[string]interface{}{"optionsReference": OptionReference{TableUID: "table", FieldUID: "field", Match: "invalid"}})
	require.ErrorIs(t, err, ErrOptionReference)
}
