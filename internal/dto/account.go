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
	// AllowPasswordSignIn being false shows only the third-party sign-in on the sign-in page, admins can still use a password.
	AllowPasswordSignIn bool `json:"allowPasswordSignIn"`
	// LoginNotice is the plain text shown above the sign-in form, empty if hidden.
	LoginNotice string `json:"loginNotice"`
	// Providers are the enabled and usable sign-in methods in the order of the admin console.
	Providers []*SiteAuthProvider `json:"providers"`
} // @name SiteInfo

// ToSiteInfo converts the site information, the redirect sign-in methods are omitted if the external URL is not set.
func ToSiteInfo(s *db.SystemSettings, providers []*db.AuthProvider) *SiteInfo {
	info := &SiteInfo{
		SiteName:            s.SiteName,
		AllowSignUp:         s.AllowSignUp && s.AllowPasswordSignIn,
		PasswordMinLength:   s.PasswordMinLength,
		AllowPasswordSignIn: s.AllowPasswordSignIn,
		LoginNotice:         s.LoginNotice,
		Providers:           make([]*SiteAuthProvider, 0, len(providers)),
	}
	for _, p := range providers {
		if p.Type.Redirect() && s.ExternalURL == "" {
			continue
		}
		info.Providers = append(info.Providers, ToSiteAuthProvider(p))
	}
	return info
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
