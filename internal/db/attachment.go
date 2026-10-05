package db

import "net/url"

// Attachment is the canonical file descriptor stored in attachment cells.
type Attachment struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	URL         string `json:"url"`
} // @name Attachment

// MaxAttachmentsPerCell bounds record payloads and the file lookups required to validate them.
const MaxAttachmentsPerCell = 100

func ToAttachment(file *File, projectUID string) Attachment {
	return Attachment{
		UID: file.UID, Name: file.Name, Size: file.Size, ContentType: file.ContentType,
		URL: "/_/projects/" + url.PathEscape(projectUID) + "/attachments/" + url.PathEscape(file.UID),
	}
}
