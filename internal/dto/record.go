package dto

import (
	"encoding/json"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Record is the schemaless table record returned by the management API.
type Record struct {
	// UID is the record UID, unique in the table.
	UID string `json:"uid"`
	// TableUID is the table of the record.
	TableUID string `json:"tableUID"`
	// Data is the cell values keyed by field UID, the system fields are not in it.
	Data map[string]interface{} `json:"data"`
	// CreatedAt is the creation time, the values of the created_at fields.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is the time of the last write, the values of the updated_at fields.
	UpdatedAt time.Time `json:"updatedAt"`
	// CreatedBy is the user or workflow creating the record, the values of the created_by fields; absent if unknown.
	CreatedBy *db.SLRecordWriter `json:"createdBy,omitempty"`
	// UpdatedBy is the user or workflow last writing the record, the values of the updated_by fields; absent if unknown.
	UpdatedBy *db.SLRecordWriter `json:"updatedBy,omitempty"`
	// CommentCount is the number of the comments, absent if none; it is always absent for the public shares.
	CommentCount int64 `json:"commentCount,omitempty"`
} // @name SLRecord

func ToRecord(table *db.SLTable, record *db.SLRecord) *Record {
	data := map[string]interface{}{}
	_ = json.Unmarshal(record.Data, &data)

	return &Record{
		UID:          record.UID,
		TableUID:     table.UID,
		Data:         data,
		CreatedAt:    record.CreatedAt,
		UpdatedAt:    record.UpdatedAt,
		CreatedBy:    writerOf(record.CreatedBy()),
		UpdatedBy:    writerOf(record.UpdatedBy()),
		CommentCount: record.CommentCount,
	}
}

func writerOf(writer db.SLRecordWriter) *db.SLRecordWriter {
	if writer.IsZero() {
		return nil
	}
	return &writer
}

func ToRecords(table *db.SLTable, records []*db.SLRecord) []*Record {
	result := make([]*Record, 0, len(records))
	for _, record := range records {
		result = append(result, ToRecord(table, record))
	}
	return result
}

// ListRecordsResp is the response of the record list and query.
type ListRecordsResp struct {
	Records []*Record `json:"records"`
	// Total is the number of records matching the query, regardless of limit and offset.
	Total int64 `json:"total"`
} // @name ListRecordsResp
