package db

import (
	"context"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/thanhpk/randstr"
	escape "github.com/tj/go-pg-escape"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLRecordsStore = (*slRecords)(nil)

// SLRecords is the default instance of the SLRecordsStore.
var SLRecords SLRecordsStore

// SLRecordsStore is the persistent interface for the records of schemaless tables.
type SLRecordsStore interface {
	// GetByID returns the record with the given ID.
	// It returns ErrSLRecordNotFound if the record does not exist.
	GetByID(ctx context.Context, slRecordID int64) (*SLRecord, error)
	// GetByUID returns the record with the given UID.
	// It returns ErrSLRecordNotFound if the record does not exist.
	GetByUID(ctx context.Context, slRecordUID string) (*SLRecord, error)
	// ListByUIDs returns the records of the table with the given UIDs in creation order, the missing ones are skipped.
	ListByUIDs(ctx context.Context, slTableID int64, uids []string) ([]*SLRecord, error)
	// ListAll returns all the records of the table in creation order.
	ListAll(ctx context.Context, slTableID int64) ([]*SLRecord, error)
	// Query returns the paginated records matching the filter, group and order options, along with the filtered total count.
	// It returns ErrSLFieldNotFound if any referenced field does not exist, ErrSLFieldNotQueryable if any referenced field is a formula,
	// and ErrUnsupportedFilterOperation, ErrInvalidFilterValue or ErrInvalidSortOrder if the options are invalid.
	Query(ctx context.Context, slTableID int64, options QuerySLRecordsOptions) ([]*SLRecord, int64, error)
	// Import creates the records in batch and returns them in the same order as the options.
	// It returns ErrSLRecordExists if any UID has been used in the table.
	Import(ctx context.Context, slTableID int64, options ImportSLRecordsOptions) ([]*SLRecord, error)
	// Create creates a record in the table with the data keyed by field UID.
	Create(ctx context.Context, slTableID int64, jsonBytes json.RawMessage) (*SLRecord, error)
	// Update replaces the data of the record with the given ID.
	Update(ctx context.Context, slRecordID int64, jsonBytes json.RawMessage) error
	// CountByTableID returns the number of records in the table.
	CountByTableID(ctx context.Context, slTableID int64) (int64, error)
	// DeleteByID deletes the record with the given ID.
	DeleteByID(ctx context.Context, slRecordID int64) error
	// DeleteByUIDs deletes the records of the table with the given UIDs, the missing ones are skipped.
	DeleteByUIDs(ctx context.Context, slTableID int64, uids []string) error
	// RemoveFieldData removes the value of the field from all the records of the table.
	RemoveFieldData(ctx context.Context, slTableID int64, fieldUID string) error
}

func NewSLRecordsStore(db *gorm.DB) SLRecordsStore {
	return &slRecords{db}
}

// SLRecord represents the table records in schemaless tables.
type SLRecord struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// SLTableID is the ID of the table the record belongs to.
	SLTableID int64 `gorm:"index;uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	// UID is the public identifier of the record unique in the table, e.g. "rec" followed by 11 random characters.
	UID string `gorm:"uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	// Data is the sparse cell values keyed by field UID, the empty values are omitted.
	Data datatypes.JSON `gorm:"type:jsonb"`
}

func (slRecord *SLRecord) BeforeCreate(_ *gorm.DB) error {
	if slRecord.UID == "" {
		slRecord.UID = "rec" + randstr.String(11)
	}
	return nil
}

type slRecords struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

func (db *slRecords) GetByID(ctx context.Context, slRecordID int64) (*SLRecord, error) {
	return db.getBy(ctx, "id = ?", slRecordID)
}

func (db *slRecords) GetByUID(ctx context.Context, slRecordUID string) (*SLRecord, error) {
	return db.getBy(ctx, "uid = ?", slRecordUID)
}

// FilterOperation is the comparison operation of a record filter.
type FilterOperation string

const (
	FilterOperationEqual              FilterOperation = "eq"
	FilterOperationNotEqual           FilterOperation = "neq"
	FilterOperationGreaterThan        FilterOperation = "gt"
	FilterOperationLessThan           FilterOperation = "lt"
	FilterOperationGreaterThanOrEqual FilterOperation = "gte"
	FilterOperationLessThanOrEqual    FilterOperation = "lte"
	FilterOperationIn                 FilterOperation = "in"
	FilterOperationNotIn              FilterOperation = "nin"
	FilterOperationLike               FilterOperation = "like"
)

// QuerySLRecordsFilter is a condition on a field value.
type QuerySLRecordsFilter struct {
	// FieldUID is the UID of the field to compare.
	FieldUID string
	// Operation is the comparison operation.
	Operation FilterOperation
	// Value is a JSON array (e.g. `["a","b"]`, `[1,2]`) for in / nin, and a single value for other operations.
	Value string
}

// QuerySLRecordsSort sorts the records by a field value.
type QuerySLRecordsSort struct {
	// FieldUID is the UID of the field to sort by.
	FieldUID string
	// Order is "asc" or "desc", empty is treated as "asc".
	Order string
}

// QuerySLRecordsGroup groups the records by a field value.
type QuerySLRecordsGroup struct {
	// FieldUID is the UID of the field to group by.
	FieldUID string
}

// QuerySLRecordsOptions are the options of querying the records.
type QuerySLRecordsOptions struct {
	// Filter are the conditions the records must all match.
	Filter []QuerySLRecordsFilter
	// Order are the sort keys applied in order, the newest records come first for ties.
	Order []QuerySLRecordsSort
	// Group sorts records by the field values before Order, so records in the same group are adjacent.
	Group []QuerySLRecordsGroup
	// Limit is the maximum number of records, it defaults to dbutil.DefaultPageSize if not positive.
	Limit int
	// Offset is the number of records to skip.
	Offset int
}

var (
	ErrSLFieldNotQueryable        = errors.New("sl_field is not queryable")
	ErrUnsupportedFilterOperation = errors.New("unsupported filter operation")
	ErrInvalidFilterValue         = errors.New("invalid filter value")
	ErrInvalidSortOrder           = errors.New("invalid sort order")
)

// Query returns the paginated records matching the filter, group and order options, along with the filtered total count.
// It returns ErrSLFieldNotFound if any referenced field does not exist.
func (db *slRecords) Query(ctx context.Context, slTableID int64, options QuerySLRecordsOptions) ([]*SLRecord, int64, error) {
	fields, err := db.getQueryFields(ctx, slTableID, options)
	if err != nil {
		return nil, 0, err
	}

	orders := make([]string, 0, len(options.Group)+len(options.Order)+1)
	for _, group := range options.Group {
		orders = append(orders, slRecordFieldExpr(fields[group.FieldUID])+" ASC NULLS LAST")
	}
	for _, sort := range options.Order {
		direction, err := parseSLRecordSortOrder(sort.Order)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, slRecordFieldExpr(fields[sort.FieldUID])+" "+direction+" NULLS LAST")
	}
	orders = append(orders, "id DESC")

	q := db.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ?", slTableID)
	for _, filter := range options.Filter {
		condition, args, err := buildSLRecordFilter(fields[filter.FieldUID], filter)
		if err != nil {
			return nil, 0, err
		}
		q = q.Where(condition, args...)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := options.Limit, options.Offset
	if limit <= 0 {
		limit = dbutil.DefaultPageSize
	}
	if offset < 0 {
		offset = 0
	}

	var records []*SLRecord
	if err := q.Order(strings.Join(orders, ", ")).Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, errors.Wrap(err, "find")
	}
	return records, count, nil
}

var slFieldUIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// getQueryFields returns the fields referenced by the query options, keyed by UID.
func (db *slRecords) getQueryFields(ctx context.Context, slTableID int64, options QuerySLRecordsOptions) (map[string]*SLField, error) {
	uids := make([]string, 0, len(options.Filter)+len(options.Order)+len(options.Group))
	for _, filter := range options.Filter {
		uids = append(uids, filter.FieldUID)
	}
	for _, sort := range options.Order {
		uids = append(uids, sort.FieldUID)
	}
	for _, group := range options.Group {
		uids = append(uids, group.FieldUID)
	}
	uids = lo.Uniq(uids)
	if len(uids) == 0 {
		return map[string]*SLField{}, nil
	}
	// UIDs are embedded into the SQL text as literals, restrict the charset to rule out quotes, gorm placeholder `?`, etc.
	for _, uid := range uids {
		if !slFieldUIDPattern.MatchString(uid) {
			return nil, ErrSLFieldNotFound
		}
	}

	var slFields []*SLField
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ? AND uid IN ?", slTableID, uids).Find(&slFields).Error; err != nil {
		return nil, errors.Wrap(err, "find fields")
	}

	fields := lo.KeyBy(slFields, func(field *SLField) string { return field.UID })
	for _, uid := range uids {
		field, ok := fields[uid]
		if !ok {
			return nil, ErrSLFieldNotFound
		}
		// Formula field values are not stored in data, so they cannot be queried directly.
		if field.Type == FormulaFieldType {
			return nil, errors.Wrapf(ErrSLFieldNotQueryable, "field %q", uid)
		}
	}
	return fields, nil
}

// slRecordFieldExpr returns the SQL expression of the field value, cast by the field type.
// Values with a mismatched JSON type are treated as NULL, so stale data won't break the cast.
func slRecordFieldExpr(field *SLField) string {
	// Do not use escape.Escape with multiple placeholders: it replaces sequentially on the whole string,
	// so a `%L` inside a previous argument would be replaced by the next one and break the quoting.
	key := escape.Literal(field.UID)
	switch field.Type {
	case NumberFieldType:
		return "(CASE WHEN jsonb_typeof(data -> " + key + ") = 'number' THEN (data ->> " + key + ")::NUMERIC END)"
	case CheckboxFieldType:
		// Unchecked records may not have the key, treat them as FALSE.
		return "COALESCE(CASE WHEN jsonb_typeof(data -> " + key + ") = 'boolean' THEN (data ->> " + key + ")::BOOLEAN END, FALSE)"
	case DateTimeFieldType:
		return "(CASE WHEN jsonb_typeof(data -> " + key + ") = 'string' THEN (data ->> " + key + ")::TIMESTAMPTZ END)"
	case MultiSelectFieldType:
		return "(data -> " + key + ")"
	default:
		return "(data ->> " + key + ")"
	}
}

var slRecordLikeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func buildSLRecordFilter(field *SLField, filter QuerySLRecordsFilter) (string, []interface{}, error) {
	expr := slRecordFieldExpr(field)
	unsupported := errors.Wrapf(ErrUnsupportedFilterOperation, "field %q (%s) operation %q", field.UID, field.Type, filter.Operation)

	// Multi-select values are arrays of option UIDs, matched with "contains" semantics.
	if field.Type == MultiSelectFieldType {
		switch filter.Operation {
		case FilterOperationEqual:
			return "COALESCE(jsonb_exists(" + expr + ", ?), FALSE)", []interface{}{filter.Value}, nil
		case FilterOperationNotEqual:
			return "NOT COALESCE(jsonb_exists(" + expr + ", ?), FALSE)", []interface{}{filter.Value}, nil
		case FilterOperationIn, FilterOperationNotIn:
			values, err := parseSLRecordFilterValues(field.Type, filter.Value)
			if err != nil {
				return "", nil, err
			}
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
			condition := "COALESCE(jsonb_exists_any(" + expr + ", ARRAY[" + placeholders + "]::TEXT[]), FALSE)"
			if filter.Operation == FilterOperationNotIn {
				condition = "NOT " + condition
			}
			return condition, values, nil
		default:
			return "", nil, unsupported
		}
	}

	switch filter.Operation {
	case FilterOperationEqual, FilterOperationNotEqual:
		value, err := parseSLRecordFilterValue(field.Type, filter.Value)
		if err != nil {
			return "", nil, err
		}
		if filter.Operation == FilterOperationEqual {
			return expr + " = ?", []interface{}{value}, nil
		}
		return expr + " IS DISTINCT FROM ?", []interface{}{value}, nil

	case FilterOperationGreaterThan, FilterOperationLessThan, FilterOperationGreaterThanOrEqual, FilterOperationLessThanOrEqual:
		if field.Type != NumberFieldType && field.Type != DateTimeFieldType && field.Type != TextFieldType {
			return "", nil, unsupported
		}
		value, err := parseSLRecordFilterValue(field.Type, filter.Value)
		if err != nil {
			return "", nil, err
		}
		operator := map[FilterOperation]string{
			FilterOperationGreaterThan:        ">",
			FilterOperationLessThan:           "<",
			FilterOperationGreaterThanOrEqual: ">=",
			FilterOperationLessThanOrEqual:    "<=",
		}[filter.Operation]
		return expr + " " + operator + " ?", []interface{}{value}, nil

	case FilterOperationIn, FilterOperationNotIn:
		values, err := parseSLRecordFilterValues(field.Type, filter.Value)
		if err != nil {
			return "", nil, err
		}
		if filter.Operation == FilterOperationIn {
			return expr + " IN ?", []interface{}{values}, nil
		}
		return "(" + expr + " IS NULL OR " + expr + " NOT IN ?)", []interface{}{values}, nil

	case FilterOperationLike:
		if field.Type != TextFieldType {
			return "", nil, unsupported
		}
		return expr + " ILIKE ?", []interface{}{"%" + slRecordLikeEscaper.Replace(filter.Value) + "%"}, nil

	default:
		return "", nil, unsupported
	}
}

// parseSLRecordFilterValue parses the filter value into a Go value matching the type of the field SQL expression.
func parseSLRecordFilterValue(fieldType SLFieldType, value string) (interface{}, error) {
	switch fieldType {
	case NumberFieldType:
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		// NaN / Inf would be interpolated as bare words by pgx under the simple protocol, causing SQL syntax errors.
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.Wrapf(ErrInvalidFilterValue, "parse %q as number", value)
		}
		return v, nil
	case CheckboxFieldType:
		v, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidFilterValue, "parse %q as bool", value)
		}
		return v, nil
	case DateTimeFieldType:
		v, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidFilterValue, "parse %q as datetime", value)
		}
		return v, nil
	default:
		return value, nil
	}
}

// parseSLRecordFilterValues parses the JSON array value of in / nin, whose elements can be strings or raw JSON values.
func parseSLRecordFilterValues(fieldType SLFieldType, value string) ([]interface{}, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(value), &raws); err != nil || len(raws) == 0 {
		return nil, errors.Wrapf(ErrInvalidFilterValue, "%q is not a non-empty JSON array", value)
	}

	values := make([]interface{}, 0, len(raws))
	for _, raw := range raws {
		s := string(raw)
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			s = str
		}
		v, err := parseSLRecordFilterValue(fieldType, s)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func parseSLRecordSortOrder(order string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "", "asc":
		return "ASC", nil
	case "desc":
		return "DESC", nil
	default:
		return "", errors.Wrapf(ErrInvalidSortOrder, "%q", order)
	}
}

func (db *slRecords) getBy(ctx context.Context, where string, args ...interface{}) (*SLRecord, error) {
	var slRecord SLRecord
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where(where, args...).First(&slRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLRecordNotFound
		}
		return nil, err
	}
	return &slRecord, nil
}

// ImportSLRecordsOptions are the options of importing records in batch.
type ImportSLRecordsOptions struct {
	// Data are the data of the records keyed by field UID, one item for each record.
	Data []json.RawMessage
	// UIDs are the UIDs of the records in the same order as Data, random UIDs are generated if it is empty.
	UIDs []string
}

// Import creates the records in batch and returns them in the same order as options.Data.
func (db *slRecords) Import(ctx context.Context, slTableID int64, options ImportSLRecordsOptions) ([]*SLRecord, error) {
	records := make([]*SLRecord, 0, len(options.Data))
	for i, jsonBytes := range options.Data {
		record := &SLRecord{
			SLTableID: slTableID,
			Data:      datatypes.JSON(jsonBytes),
		}
		if i < len(options.UIDs) {
			record.UID = options.UIDs[i]
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return records, nil
	}

	if err := db.WithContext(ctx).Create(&records).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_table_id_uid") {
			return nil, ErrSLRecordExists
		}
		return nil, errors.Wrap(err, "create")
	}
	return records, nil
}

func (db *slRecords) Create(ctx context.Context, slTableID int64, jsonBytes json.RawMessage) (*SLRecord, error) {
	slRecord := &SLRecord{
		SLTableID: slTableID,
		Data:      datatypes.JSON(jsonBytes),
	}
	if err := db.WithContext(ctx).Create(slRecord).Error; err != nil {
		return nil, errors.Wrap(err, "create")
	}
	return slRecord, nil
}

var (
	ErrSLRecordNotFound = errors.New("sl_record does not exist")
	ErrSLRecordExists   = errors.New("sl_record exists")
)

func (db *slRecords) ListByUIDs(ctx context.Context, slTableID int64, uids []string) ([]*SLRecord, error) {
	var records []*SLRecord
	if len(uids) == 0 {
		return records, nil
	}
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ? AND uid IN ?", slTableID, uids).Order("id ASC").Find(&records).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return records, nil
}

func (db *slRecords) ListAll(ctx context.Context, slTableID int64) ([]*SLRecord, error) {
	var records []*SLRecord
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ?", slTableID).Order("id ASC").Find(&records).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return records, nil
}

func (db *slRecords) DeleteByUIDs(ctx context.Context, slTableID int64, uids []string) error {
	if len(uids) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Where("sl_table_id = ? AND uid IN ?", slTableID, uids).Delete(&SLRecord{}).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}

func (db *slRecords) RemoveFieldData(ctx context.Context, slTableID int64, fieldUID string) error {
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ? AND jsonb_exists(data, ?)", slTableID, fieldUID).
		Update("data", gorm.Expr("data - ?::TEXT", fieldUID)).Error; err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

func (db *slRecords) Update(ctx context.Context, slRecordID int64, jsonBytes json.RawMessage) error {
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("id = ?", slRecordID).Updates(map[string]interface{}{
		"data": jsonBytes,
	}).Error; err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

func (db *slRecords) CountByTableID(ctx context.Context, slTableID int64) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ?", slTableID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return count, nil
}

func (db *slRecords) DeleteByID(ctx context.Context, slRecordID int64) error {
	if err := db.WithContext(ctx).Model(&SLRecord{}).Delete(&SLRecord{}, slRecordID).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}
