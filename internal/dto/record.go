package dto

import (
	"encoding/json"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Record is the schemaless table record returned by the management API.
type Record struct {
	UID      string `json:"uid"`
	TableUID string `json:"tableUID"`
	// Data is the cell values keyed by field UID.
	Data      map[string]interface{} `json:"data"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
} // @name SLRecord

func ToRecord(table *db.SLTable, record *db.SLRecord) *Record {
	data := map[string]interface{}{}
	_ = json.Unmarshal(record.Data, &data)

	return &Record{
		UID:       record.UID,
		TableUID:  table.UID,
		Data:      data,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
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
