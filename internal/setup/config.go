package setup

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/cockroachdb/errors"
)

// PostgresOptions are the PostgreSQL connection entered in the wizard.
type PostgresOptions struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
}

// SSLModes are the supported libpq sslmode values.
var SSLModes = []string{"disable", "require", "verify-ca", "verify-full"}

// DSN returns the connection URL, escaping the credentials and the database name.
func (o PostgresOptions) DSN() string {
	user := url.User(o.User)
	if o.Password != "" {
		user = url.UserPassword(o.User, o.Password)
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     net.JoinHostPort(o.Host, strconv.Itoa(o.Port)),
		Path:     "/" + o.Database,
		RawQuery: url.Values{"sslmode": {o.SSLMode}}.Encode(),
	}
	return u.String()
}

// RedisOptions are the Redis connection entered in the wizard.
type RedisOptions struct {
	Address  string
	Username string
	Password string
}

// DatabaseOptions are the services configured in the database step.
type DatabaseOptions struct {
	Postgres PostgresOptions
	Redis    RedisOptions
}

// ConfigOptions are the values of the generated configuration file.
type ConfigOptions struct {
	Postgres  PostgresOptions
	Redis     RedisOptions
	SecretKey string
}

//go:embed sayrud.yaml.tmpl
var configTemplateText string

var configTemplate = template.Must(template.New("config").Funcs(template.FuncMap{"quote": quoteYAML}).Parse(configTemplateText))

// quoteYAML returns the value as a double-quoted scalar, JSON strings are valid YAML.
func quoteYAML(value string) (string, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// RenderConfig renders the configuration file with the options and the defaults of config/sayrud.example.yaml.
func RenderConfig(opts ConfigOptions) ([]byte, error) {
	data := struct {
		Postgres  struct{ DSN string }
		Redis     RedisOptions
		SecretKey string
	}{Redis: opts.Redis, SecretKey: opts.SecretKey}
	data.Postgres.DSN = opts.Postgres.DSN()

	var b bytes.Buffer
	if err := configTemplate.Execute(&b, data); err != nil {
		return nil, errors.Wrap(err, "execute template")
	}
	return b.Bytes(), nil
}

// GenerateSecretKey returns the base64 of 32 random bytes for auth.secret_key.
func GenerateSecretKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", errors.Wrap(err, "read random bytes")
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// ErrConfigExists is returned when committing a configuration file that has been created by someone else.
var ErrConfigExists = errors.New("configuration file already exists")

// PendingConfig is a configuration written next to its final path, so that it can be loaded and verified before taking effect.
type PendingConfig struct {
	path string
	temp string
	done bool
}

// WriteConfig writes the data to a temporary file readable only by the owner in the directory of path.
func WriteConfig(path string, data []byte) (*PendingConfig, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, errors.Wrap(err, "create directory")
	}

	// Keep the extension, the configuration loader detects the format by it.
	ext := filepath.Ext(path)
	file, err := os.CreateTemp(dir, "."+strings.TrimSuffix(filepath.Base(path), ext)+".setup-*"+ext)
	if err != nil {
		return nil, errors.Wrap(err, "create temporary file")
	}
	pending := &PendingConfig{path: path, temp: file.Name()}

	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		pending.Discard()
		return nil, errors.Wrap(err, "write temporary file")
	}
	return pending, nil
}

// TempPath returns the path of the temporary file.
func (p *PendingConfig) TempPath() string {
	return p.temp
}

// Commit publishes the configuration without replacing an existing file, using an atomic hard link where supported.
func (p *PendingConfig) Commit() error {
	// A hard link fails if the target exists, which a rename would silently replace.
	err := os.Link(p.temp, p.path)
	switch {
	case err == nil:
		_ = os.Remove(p.temp)
	case errors.Is(err, os.ErrExist):
		return ErrConfigExists
	default:
		if err := p.copyExclusive(); err != nil {
			return err
		}
	}

	p.done = true

	if dir, err := os.Open(filepath.Dir(p.path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}

	return nil
}

// copyExclusive supports file systems without hard links while still refusing to overwrite another installer's configuration.
func (p *PendingConfig) copyExclusive() error {
	source, err := os.Open(p.temp)
	if err != nil {
		return errors.Wrap(err, "open temporary file")
	}
	defer func() { _ = source.Close() }()

	target, err := os.OpenFile(p.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return ErrConfigExists
	}
	if err != nil {
		return errors.Wrap(err, "create configuration file")
	}

	_, err = io.Copy(target, source)
	if err == nil {
		err = target.Sync()
	}
	if closeErr := target.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(p.path)
		return errors.Wrap(err, "copy configuration file")
	}

	_ = os.Remove(p.temp)
	return nil
}

// Discard removes the temporary file if it has not been committed.
func (p *PendingConfig) Discard() {
	if !p.done {
		_ = os.Remove(p.temp)
		p.done = true
	}
}
