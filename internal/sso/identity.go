package sso

import (
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// Identity is the third-party account converted from any protocol.
type Identity struct {
	// Subject is the stable and unique ID of the account in the provider.
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Groups        []string
}

// Error codes of failed sign-ins, returned to the frontend in the sso_error query for redirect sign-ins, or translated as the sso::<code> messages for LDAP.
const (
	CodeProviderNotFound = "provider_not_found"
	CodeNotConfigured    = "sso_not_configured"
	CodeInvalidState     = "invalid_state"
	CodeIdPError         = "idp_error"
	CodeAccessDenied     = "access_denied"
	CodeNotLinked        = "not_linked"
	CodeEmailRequired    = "email_required"
	CodeEmailTaken       = "email_taken"
	CodeIdentityTaken    = "identity_taken"
	CodeAccountDisabled  = "account_disabled"
)

// Error is a failed sign-in with the error code, Err is the detailed reason for the logs.
type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return e.Code + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Errorf returns the *Error of the code.
func Errorf(code string, err error) error {
	return &Error{Code: code, Err: err}
}

// ErrorCode returns the code of the *Error in the chain of err, or CodeIdPError if there is none.
func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeIdPError
}

var (
	// ErrBadCredential means the LDAP user does not exist or the password is wrong.
	ErrBadCredential = errors.New("bad credential")
	errNoCertificate = errors.New("no certificate found")
)

// EmailDomain returns the lowercase domain after the @ of the email.
func EmailDomain(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

// CheckAccess reports whether the account matches the allowed email domains and groups, an empty list allows all.
func CheckAccess(id *Identity, domains, groups []string) bool {
	if len(domains) > 0 && !contains(domains, EmailDomain(id.Email)) {
		return false
	}
	if len(groups) > 0 {
		for _, g := range id.Groups {
			if contains(groups, g) {
				return true
			}
		}
		return false
	}
	return true
}

// NormalizeDomains lowercases the domains, trims the leading @ and spaces, and removes the duplicates.
func NormalizeDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "@"))
		if d != "" && !contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

// NormalizeGroups trims the spaces and removes the duplicates, the group names are case-sensitive.
func NormalizeGroups(groups []string) []string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" && !contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

const userNameMaxLength = 32

// DeriveUserName returns the user name of the automatically created account: Name, the part of the email before @, or Subject in order, truncated to 32 characters.
func DeriveUserName(id *Identity) string {
	name := strings.TrimSpace(id.Name)
	if name == "" {
		if at := strings.IndexByte(id.Email, '@'); at > 0 {
			name = id.Email[:at]
		}
	}
	if name == "" {
		name = id.Subject
	}
	if utf8.RuneCountInString(name) > userNameMaxLength {
		name = string([]rune(name)[:userNameMaxLength])
	}
	return name
}
