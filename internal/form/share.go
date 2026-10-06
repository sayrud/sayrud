package form

type UpdateLinkShare struct {
	Enabled         bool `json:"enabled"`
	IncludeChildren bool `json:"includeChildren"`
	// Nil preserves the password, an empty string disables it.
	Password *string `json:"password,omitempty"`
} // @name UpdateLinkShare

type UnlockLinkShare struct {
	Password string `json:"password"`
} // @name UnlockLinkShare
