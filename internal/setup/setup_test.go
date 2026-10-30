package setup

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/conf"
)

func TestNewToken(t *testing.T) {
	token, err := NewToken()
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[A-Z2-7]{4}(-[A-Z2-7]{4}){5}$`), token.String())

	assert.True(t, token.Verify(token.String()))
	assert.True(t, token.Verify("  "+strings.ReplaceAll(token.String(), "-", " ")+"\n"), "spaces and dashes are ignored")
	assert.True(t, token.Verify(strings.ToLower(token.String())), "case is ignored")
	assert.False(t, token.Verify(""))
	assert.False(t, token.Verify(token.String()[:len(token.String())-1]))

	other, err := NewToken()
	require.NoError(t, err)
	assert.NotEqual(t, token.String(), other.String())
	assert.False(t, token.Verify(other.String()))
}

func TestParseToken(t *testing.T) {
	_, err := ParseToken("short-token")
	require.Error(t, err)
	_, err = ParseToken("abcd-efgh-ijkl-mno")
	require.Error(t, err, "dashes do not count towards the minimum length")

	token, err := ParseToken(" my-deployment-secret-2026 ")
	require.NoError(t, err)
	assert.Equal(t, "my-deployment-secret-2026", token.String())
	assert.True(t, token.Verify("MYDEPLOYMENTSECRET2026"))
	assert.False(t, token.Verify("my-deployment-secret-2025"))
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	assert.True(t, l.Allow("1.1.1.1"))
	l.Fail("1.1.1.1")
	assert.True(t, l.Allow("1.1.1.1"))
	l.Fail("1.1.1.1")
	assert.False(t, l.Allow("1.1.1.1"))
	assert.True(t, l.Allow("2.2.2.2"), "the clients are counted separately")

	now = now.Add(time.Minute)
	assert.True(t, l.Allow("1.1.1.1"), "the window expires")
	l.Fail("1.1.1.1")
	assert.True(t, l.Allow("1.1.1.1"), "a new window starts after expiring")
}

func TestPostgresDSN(t *testing.T) {
	dsn := PostgresOptions{
		Host: "db.internal", Port: 5433, Database: "say rud", User: "app@corp", Password: "p@ss:w/rd?#", SSLMode: "require",
	}.DSN()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	assert.Equal(t, "postgres", u.Scheme)
	assert.Equal(t, "db.internal:5433", u.Host)
	assert.Equal(t, "/say rud", u.Path)
	assert.Equal(t, "app@corp", u.User.Username())
	password, _ := u.User.Password()
	assert.Equal(t, "p@ss:w/rd?#", password)
	assert.Equal(t, "require", u.Query().Get("sslmode"))

	ipv6 := PostgresOptions{Host: "::1", Port: 5432, Database: "sayrud", User: "postgres", SSLMode: "disable"}.DSN()
	assert.Equal(t, "postgres://postgres@[::1]:5432/sayrud?sslmode=disable", ipv6)
}

func TestRenderConfig(t *testing.T) {
	previousApp, previousPostgres, previousRedis, previousAuth := conf.App, conf.Postgres, conf.Redis, conf.Auth
	t.Cleanup(func() {
		conf.App, conf.Postgres, conf.Redis, conf.Auth = previousApp, previousPostgres, previousRedis, previousAuth
	})

	secretKey, err := GenerateSecretKey()
	require.NoError(t, err)
	opts := ConfigOptions{
		Postgres: PostgresOptions{
			Host: "127.0.0.1", Port: 5432, Database: "sayrud", User: "postgres", Password: `quote"back\slash#hash: colon`, SSLMode: "disable",
		},
		Redis:     RedisOptions{Address: "127.0.0.1:6379", Username: "default", Password: "'single' #not-a-comment"},
		SecretKey: secretKey,
	}
	data, err := RenderConfig(opts)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "sayrud.yaml")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	require.NoError(t, conf.Init(path))
	assert.Equal(t, opts.Postgres.DSN(), conf.Postgres.DSN)
	assert.Equal(t, opts.Redis.Address, conf.Redis.Address)
	assert.Equal(t, opts.Redis.Username, conf.Redis.Username)
	assert.Equal(t, opts.Redis.Password, conf.Redis.Password)
	assert.Equal(t, secretKey, conf.Auth.SecretKey)

	assert.Equal(t, configKeys(t, "../../config/sayrud.example.yaml"), configKeys(t, path), "the generated configuration has the same options as the example")
}

func configKeys(t *testing.T, path string) []string {
	t.Helper()
	v := viper.New()
	v.SetConfigFile(path)
	require.NoError(t, v.ReadInConfig())
	keys := v.AllKeys()
	sort.Strings(keys)
	return keys
}

func TestWriteConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	path := filepath.Join(dir, "sayrud.yaml")

	pending, err := WriteConfig(path, []byte("app:\n  drain_delay: 5s\n"))
	require.NoError(t, err)
	assert.Equal(t, ".yaml", filepath.Ext(pending.TempPath()), "the loader detects the format by the extension")
	info, err := os.Stat(pending.TempPath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist, "the configuration only appears after committing")

	require.NoError(t, pending.Commit())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "app:\n  drain_delay: 5s\n", string(content))
	pending.Discard()
	_, err = os.Stat(path)
	require.NoError(t, err, "discarding after committing keeps the configuration")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the temporary file is removed")

	another, err := WriteConfig(path, []byte("app:\n  drain_delay: 6s\n"))
	require.NoError(t, err)
	require.ErrorIs(t, another.Commit(), ErrConfigExists)
	another.Discard()
	content, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "app:\n  drain_delay: 5s\n", string(content), "an existing configuration is never replaced")
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
