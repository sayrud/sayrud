package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// SiteInfo is the public site information readable without signing in.
type SiteInfo struct {
	SiteName          string `json:"siteName"`
	AllowSignUp       bool   `json:"allowSignUp"`
	PasswordMinLength int    `json:"passwordMinLength"`
} // @name SiteInfo

func ToSiteInfo(s *db.SystemSettings) *SiteInfo {
	return &SiteInfo{
		SiteName:          s.SiteName,
		AllowSignUp:       s.AllowSignUp,
		PasswordMinLength: s.PasswordMinLength,
	}
}

// UserSession is a signed-in device of the user.
type UserSession struct {
	ID        int64     `json:"id"`
	UserAgent string    `json:"userAgent"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	// Current reports whether it is the session of the current request.
	Current bool `json:"current"`
} // @name UserSession

func ToUserSession(s *db.UserSession, currentID int64) *UserSession {
	return &UserSession{
		ID:        s.ID,
		UserAgent: s.UserAgent,
		IP:        s.IP,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
		Current:   s.ID == currentID,
	}
}
