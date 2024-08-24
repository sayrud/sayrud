// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package apibuilder

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
)

type Datasets []Dataset

func (d Datasets) ValidateConfig(ctx context.Context) error {
	if len(d) == 0 {
		return ErrEmptyDatasets
	}

	for _, dataset := range d {
		if err := dataset.ValidateConfig(ctx); err != nil {
			return err
		}
	}
	return nil
}

var _ ConfigValidator = (*Dataset)(nil)

type Dataset struct {
	TableUID         string    `json:"tableUID"`
	Fields           []string  `json:"fields"`
	Filter           *Operator `json:"filter"`
	Orders           []*Order  `json:"order"`
	LimitExpression  string    `json:"limitExp"`
	OffsetExpression string    `json:"offsetExp"`
}

func (d Dataset) ValidateConfig(ctx context.Context) error {
	projectID := ParseProjectID(ctx)

	if d.TableUID == "" {
		return ErrEmptyDatasetTableUID
	}

	// Check the table UID is valid.
	table, err := db.SLTables.GetByUID(ctx, d.TableUID)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ErrDatasetTableNotFound
		}
		return errors.Wrap(err, "get sl table by UID")
	}
	if table.ProjectID != projectID {
		return ErrDatasetTableNotFound
	}

	// Skip the fields validation if the kind is create or update or delete.
	kind := ParseKind(ctx)
	if kind == KindCreate || kind == KindUpdate || kind == KindDelete {
		return nil
	}

	if len(d.Fields) == 0 {
		return ErrEmptyDatasetFields
	}
	// Check the fields UID belongs to the table.
	slFields, err := db.SLFields.GetByTableID(ctx, table.ID)
	if err != nil {
		return errors.Wrap(err, "get sl fields by table ID")
	}
	slFieldUIDs := lo.Map(slFields, func(f *db.SLField, _ int) string {
		return f.UID
	})
	for _, fieldUID := range d.Fields {
		if !lo.Contains(slFieldUIDs, fieldUID) && fieldUID != "_uid" {
			return ErrFieldNotFound
		}
	}

	return nil
}

func (d Dataset) ToJSON() []byte {
	b, _ := json.Marshal(d)
	return b
}
