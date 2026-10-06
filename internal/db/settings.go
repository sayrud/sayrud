package db

import (
	"context"
	"encoding/json"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
)

var _ SettingsStore = (*settings)(nil)

// Settings is the default instance of the SettingsStore.
var Settings SettingsStore

// SettingsStore is the persistent interface for the system settings and the settings of users.
type SettingsStore interface {
	// GetSystem returns the system settings, the unsaved ones take the defaults.
	GetSystem(ctx context.Context) (*SystemSettings, error)
	// SaveSystem saves the system settings, the caller must Validate them first.
	SaveSystem(ctx context.Context, s SystemSettings) error
	// GetAI returns the AI settings, the unsaved ones take the defaults.
	GetAI(ctx context.Context) (*AISettings, error)
	// SaveAI saves the AI settings, the caller must Validate them first.
	SaveAI(ctx context.Context, s AISettings) error
	// GetUser returns the settings of the user, the unsaved ones take the defaults.
	GetUser(ctx context.Context, userID int64) (*UserSettings, error)
	// SaveUser saves the settings of the user, the caller must Validate them first.
	SaveUser(ctx context.Context, userID int64, s UserSettings) error
}

func NewSettingsStore(db *gorm.DB) SettingsStore {
	return &settings{db}
}

// SystemSettingsUserID is the user ID of the system settings in the settings table.
const SystemSettingsUserID int64 = 0

// Setting is a setting of the system or a user, one row for each key.
type Setting struct {
	ID int64 `gorm:"primarykey"`
	// UserID is the owner of the setting, SystemSettingsUserID for the system settings.
	UserID    int64          `gorm:"not null;default:0;uniqueIndex:idx_settings_user_key"`
	Key       string         `gorm:"type:varchar(64);not null;uniqueIndex:idx_settings_user_key"`
	Value     datatypes.JSON `gorm:"type:jsonb;not null"`
	UpdatedAt time.Time
}

type settings struct {
	*gorm.DB
}

// load returns the settings of the user as a JSON object, which can be unmarshalled on top of the defaults.
func (db *settings) load(ctx context.Context, userID int64) ([]byte, error) {
	var rows []*Setting
	if err := db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make(map[string]json.RawMessage, len(rows))
	for _, row := range rows {
		values[row.Key] = json.RawMessage(row.Value)
	}
	return settingsObject(values), nil
}

// save upserts a row for each field of the settings struct.
func (db *settings) save(ctx context.Context, userID int64, s interface{}) error {
	values, err := settingValues(s)
	if err != nil {
		return err
	}
	now := dbutil.Now()
	rows := make([]*Setting, 0, len(values))
	for key, value := range values {
		rows = append(rows, &Setting{UserID: userID, Key: key, Value: datatypes.JSON(value), UpdatedAt: now})
	}
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&rows).Error
}

// settingValues splits the settings struct into the JSON values keyed by the JSON field names.
func settingValues(s interface{}) (map[string]json.RawMessage, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, errors.Wrap(err, "unmarshal")
	}
	return values, nil
}

// settingsObject joins the JSON values keyed by the field names into a JSON object.
func settingsObject(values map[string]json.RawMessage) []byte {
	raw, _ := json.Marshal(values)
	return raw
}

// SystemSettings are the system settings editable in the admin console.
type SystemSettings struct {
	// SiteName is shown in the page title and the sign-in page.
	SiteName string `json:"siteName"`
	// AllowSignUp reports whether new users can sign up by themselves.
	AllowSignUp bool `json:"allowSignUp"`
	// PasswordMinLength applies to signing up, changing, creating and resetting passwords.
	PasswordMinLength int `json:"passwordMinLength"`
	// SessionTTLDays is the lifetime in days of the new sessions.
	SessionTTLDays int `json:"sessionTTLDays"`
	// ExternalURL is the address users visit (e.g. https://sayrud.example.com), the callback URLs of third-party sign-in are built on it, empty if not set.
	ExternalURL string `json:"externalURL"`
	// AllowPasswordSignIn being false allows only the admins to sign in with email and password.
	AllowPasswordSignIn bool `json:"allowPasswordSignIn"`
	// LoginNotice is the plain text shown above the sign-in form, empty if hidden.
	LoginNotice string `json:"loginNotice"`
	// NetworkAllowlist permits shortcut fetches to these otherwise blocked IP addresses or CIDRs.
	NetworkAllowlist []string `json:"networkAllowlist"`
} // @name SystemSettings

const (
	SiteNameMaxLength    = 32
	LoginNoticeMaxLength = 500
	PasswordMinLengthMin = 8
	PasswordMinLengthMax = 64
	SessionTTLDaysMin    = 1
	SessionTTLDaysMax    = 365
)

// DefaultSystemSettings returns the defaults before any settings are saved, signing up follows the config file.
func DefaultSystemSettings() SystemSettings {
	return SystemSettings{
		SiteName:            "Sayrud",
		AllowSignUp:         !conf.Auth.DisableSignUp,
		PasswordMinLength:   8,
		SessionTTLDays:      30,
		AllowPasswordSignIn: true,
		NetworkAllowlist:    []string{},
	}
}

// ParseNetworkAllowlist accepts IPv4/IPv6 addresses and CIDRs, and normalizes mapped IPv4 and host bits.
func ParseNetworkAllowlist(entries []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		value := strings.TrimSpace(entry)
		if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" {
			addr = addr.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, i18n.Errorf("settings::invalid_network_allowlist", entry)
		}
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

// NormalizeExternalURL returns the URL without the trailing /, empty is valid, it only accepts http(s) URLs without path, query and user info.
func NormalizeExternalURL(raw string) (string, bool) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// ParseSystemSettings applies the JSON object on top of the defaults, the missing, invalid or out-of-range fields take the defaults.
func ParseSystemSettings(raw []byte, defaults SystemSettings) SystemSettings {
	s := defaults
	if len(raw) > 0 && json.Unmarshal(raw, &s) != nil {
		return defaults
	}
	s.SiteName = strings.TrimSpace(s.SiteName)
	if s.SiteName == "" || utf8.RuneCountInString(s.SiteName) > SiteNameMaxLength {
		s.SiteName = defaults.SiteName
	}
	if s.PasswordMinLength < PasswordMinLengthMin || s.PasswordMinLength > PasswordMinLengthMax {
		s.PasswordMinLength = defaults.PasswordMinLength
	}
	if s.SessionTTLDays < SessionTTLDaysMin || s.SessionTTLDays > SessionTTLDaysMax {
		s.SessionTTLDays = defaults.SessionTTLDays
	}
	if u, ok := NormalizeExternalURL(s.ExternalURL); ok {
		s.ExternalURL = u
	} else {
		s.ExternalURL = defaults.ExternalURL
	}
	s.LoginNotice = strings.TrimSpace(s.LoginNotice)
	if utf8.RuneCountInString(s.LoginNotice) > LoginNoticeMaxLength {
		s.LoginNotice = defaults.LoginNotice
	}
	if _, err := ParseNetworkAllowlist(s.NetworkAllowlist); err != nil || s.NetworkAllowlist == nil {
		s.NetworkAllowlist = defaults.NetworkAllowlist
	}
	return s
}

// Validate returns the *i18n.Error to be translated.
func (s SystemSettings) Validate() error {
	if n := utf8.RuneCountInString(strings.TrimSpace(s.SiteName)); n == 0 || n > SiteNameMaxLength {
		return i18n.Errorf("settings::site_name_length", SiteNameMaxLength)
	}
	if s.PasswordMinLength < PasswordMinLengthMin || s.PasswordMinLength > PasswordMinLengthMax {
		return i18n.Errorf("settings::password_min_length_range", PasswordMinLengthMin, PasswordMinLengthMax)
	}
	if s.SessionTTLDays < SessionTTLDaysMin || s.SessionTTLDays > SessionTTLDaysMax {
		return i18n.Errorf("settings::session_ttl_range", SessionTTLDaysMin, SessionTTLDaysMax)
	}
	if _, ok := NormalizeExternalURL(s.ExternalURL); !ok {
		return i18n.Errorf("settings::invalid_external_url")
	}
	if utf8.RuneCountInString(strings.TrimSpace(s.LoginNotice)) > LoginNoticeMaxLength {
		return i18n.Errorf("settings::login_notice_length", LoginNoticeMaxLength)
	}
	_, err := ParseNetworkAllowlist(s.NetworkAllowlist)
	return err
}

// SessionTTL returns the lifetime of the new sessions.
func (s SystemSettings) SessionTTL() time.Duration {
	return time.Duration(s.SessionTTLDays) * 24 * time.Hour
}

func (db *settings) GetSystem(ctx context.Context) (*SystemSettings, error) {
	raw, err := db.load(ctx, SystemSettingsUserID)
	if err != nil {
		return nil, err
	}
	s := ParseSystemSettings(raw, DefaultSystemSettings())
	return &s, nil
}

func (db *settings) SaveSystem(ctx context.Context, s SystemSettings) error {
	s.SiteName = strings.TrimSpace(s.SiteName)
	s.LoginNotice = strings.TrimSpace(s.LoginNotice)
	s.ExternalURL, _ = NormalizeExternalURL(s.ExternalURL)
	return db.save(ctx, SystemSettingsUserID, s)
}

// AISettings is the global OpenAI-compatible Chat Completions model configuration, edited in the admin console.
type AISettings struct {
	// Enabled reports whether the global AI model is enabled.
	Enabled bool `json:"enabled"`
	// BaseURL is the API root including the version, e.g. https://api.openai.com/v1.
	BaseURL string `json:"baseURL"`
	// Model is the name of the model to call.
	Model string `json:"model"`
	// SealedAPIKey is the API key encrypted by auth.secret_key, empty sends the requests without authorization.
	SealedAPIKey string `json:"sealedAPIKey"`
	// TimeoutSeconds is the timeout of a completion request.
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// aiSettingsRow stores the AI settings as a single row with the key "ai" in the system settings.
type aiSettingsRow struct {
	AI AISettings `json:"ai"`
}

const (
	AIModelMaxLength    = 128
	AITimeoutSecondsMin = 1
	// AITimeoutSecondsMax limits AI completion requests to 2 minutes.
	AITimeoutSecondsMax = 120
)

// DefaultAISettings returns the defaults before the AI settings are saved.
func DefaultAISettings() AISettings {
	return AISettings{BaseURL: "https://api.openai.com/v1", TimeoutSeconds: 60}
}

// Configured reports whether the AI model is enabled with the API URL and the model filled in.
func (s AISettings) Configured() bool {
	return s.Enabled && s.BaseURL != "" && s.Model != ""
}

// Timeout returns the timeout of a completion request.
func (s AISettings) Timeout() time.Duration {
	return time.Duration(s.TimeoutSeconds) * time.Second
}

// NormalizeAIBaseURL returns the URL without the trailing /, empty is valid, it only accepts http(s) URLs without user info, query and fragment.
func NormalizeAIBaseURL(raw string) (string, bool) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return raw, true
}

// ParseAISettings applies the JSON object on top of the defaults, the invalid or out-of-range fields take the defaults.
func ParseAISettings(raw []byte, defaults AISettings) AISettings {
	s := defaults
	if len(raw) > 0 && json.Unmarshal(raw, &s) != nil {
		return defaults
	}

	if u, ok := NormalizeAIBaseURL(s.BaseURL); ok {
		s.BaseURL = u
	} else {
		s.BaseURL = defaults.BaseURL
	}
	s.Model = strings.TrimSpace(s.Model)
	if utf8.RuneCountInString(s.Model) > AIModelMaxLength {
		s.Model = defaults.Model
	}
	if s.TimeoutSeconds < AITimeoutSecondsMin || s.TimeoutSeconds > AITimeoutSecondsMax {
		s.TimeoutSeconds = defaults.TimeoutSeconds
	}
	return s
}

// Validate returns the *i18n.Error to be translated.
func (s AISettings) Validate() error {
	baseURL, ok := NormalizeAIBaseURL(s.BaseURL)
	if !ok {
		return i18n.Errorf("ai::invalid_base_url")
	}
	model := strings.TrimSpace(s.Model)
	if utf8.RuneCountInString(model) > AIModelMaxLength {
		return i18n.Errorf("ai::model_too_long", AIModelMaxLength)
	}
	if s.Enabled && baseURL == "" {
		return i18n.Errorf("ai::base_url_required")
	}
	if s.Enabled && model == "" {
		return i18n.Errorf("ai::model_required")
	}
	if s.TimeoutSeconds < AITimeoutSecondsMin || s.TimeoutSeconds > AITimeoutSecondsMax {
		return i18n.Errorf("ai::invalid_timeout", AITimeoutSecondsMin, AITimeoutSecondsMax)
	}
	return nil
}

func (db *settings) GetAI(ctx context.Context) (*AISettings, error) {
	raw, err := db.load(ctx, SystemSettingsUserID)
	if err != nil {
		return nil, err
	}
	var row struct {
		AI json.RawMessage `json:"ai"`
	}
	_ = json.Unmarshal(raw, &row)
	s := ParseAISettings(row.AI, DefaultAISettings())
	return &s, nil
}

func (db *settings) SaveAI(ctx context.Context, s AISettings) error {
	s.BaseURL, _ = NormalizeAIBaseURL(s.BaseURL)
	s.Model = strings.TrimSpace(s.Model)
	return db.save(ctx, SystemSettingsUserID, aiSettingsRow{AI: s})
}

// The appearances of the user interface.
const (
	ThemeLight  = "light"
	ThemeDark   = "dark"
	ThemeSystem = "system"
)

// UserSettings are the personal settings of a user, synced across the devices.
type UserSettings struct {
	// Theme is the appearance of the user interface.
	Theme string `json:"theme" enums:"light,dark,system"`
	// Language is the language of the user interface.
	Language string `json:"language" enums:"en,zh-CN,zh-TW,ja,ko,es,pt-BR,fr,de,ru"`
} // @name UserSettings

// DefaultUserSettings returns the defaults before the user saves any settings.
func DefaultUserSettings() UserSettings {
	return UserSettings{Theme: ThemeLight, Language: i18n.LangZhCN}
}

func isTheme(theme string) bool {
	return theme == ThemeLight || theme == ThemeDark || theme == ThemeSystem
}

// ParseUserSettings applies the JSON object on top of the defaults, the missing or invalid fields take the defaults.
func ParseUserSettings(raw []byte, defaults UserSettings) UserSettings {
	s := defaults
	if len(raw) > 0 && json.Unmarshal(raw, &s) != nil {
		return defaults
	}
	if !isTheme(s.Theme) {
		s.Theme = defaults.Theme
	}
	if !i18n.IsSupported(s.Language) {
		s.Language = defaults.Language
	}
	return s
}

// Validate returns the *i18n.Error to be translated.
func (s UserSettings) Validate() error {
	if !isTheme(s.Theme) {
		return i18n.Errorf("settings::invalid_theme")
	}
	if !i18n.IsSupported(s.Language) {
		return i18n.Errorf("settings::invalid_language")
	}
	return nil
}

// Apply returns the settings overridden by the non-nil fields.
func (s UserSettings) Apply(theme, language *string) UserSettings {
	if theme != nil {
		s.Theme = *theme
	}
	if language != nil {
		s.Language = *language
	}
	return s
}

func (db *settings) GetUser(ctx context.Context, userID int64) (*UserSettings, error) {
	raw, err := db.load(ctx, userID)
	if err != nil {
		return nil, err
	}
	s := ParseUserSettings(raw, DefaultUserSettings())
	return &s, nil
}

func (db *settings) SaveUser(ctx context.Context, userID int64, s UserSettings) error {
	return db.save(ctx, userID, s)
}
