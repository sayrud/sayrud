package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// AccessToken is a personal access token, the plaintext is only returned on creation.
type AccessToken struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Prefix string   `json:"prefix"`
	Scopes []string `json:"scopes" enums:"read,write,events"`
	// Projects are the projects the token is restricted to, empty if it follows the memberships of the user.
	Projects []*AccessTokenProject `json:"projects"`
	// ExpiresAt is nil if the token never expires.
	ExpiresAt  *time.Time `json:"expiresAt"`
	Expired    bool       `json:"expired"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	LastUsedIP string     `json:"lastUsedIP"`
	CreatedAt  time.Time  `json:"createdAt"`
} // @name AccessToken

// AccessTokenProject is a project an access token is restricted to.
type AccessTokenProject struct {
	UID   string `json:"uid"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
	// Deleted reports whether the project has been deleted, the token can no longer access it.
	Deleted bool `json:"deleted"`
} // @name AccessTokenProject

// ToAccessToken converts the token, the projects are keyed by ID and the missing ones are deleted.
func ToAccessToken(t *db.AccessToken, projects map[int64]*db.Project, now time.Time) *AccessToken {
	scopes := make([]string, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, string(s))
	}
	restricted := make([]*AccessTokenProject, 0, len(t.ProjectIDs))
	for _, id := range t.ProjectIDs {
		if p, ok := projects[id]; ok {
			restricted = append(restricted, &AccessTokenProject{UID: p.UID, Name: p.Name, Icon: p.Icon, Color: p.Color})
		} else {
			restricted = append(restricted, &AccessTokenProject{Deleted: true})
		}
	}
	return &AccessToken{
		ID:         t.ID,
		Name:       t.Name,
		Prefix:     t.Prefix,
		Scopes:     scopes,
		Projects:   restricted,
		ExpiresAt:  t.ExpiresAt,
		Expired:    t.Expired(now),
		LastUsedAt: t.LastUsedAt,
		LastUsedIP: t.LastUsedIP,
		CreatedAt:  t.CreatedAt,
	}
}

// CreateAccessTokenResp returns the new token and its plaintext.
type CreateAccessTokenResp struct {
	Token *AccessToken `json:"token"`
	// Secret is the plaintext of the token, it can not be retrieved again.
	Secret string `json:"secret"`
} // @name CreateAccessTokenResp
