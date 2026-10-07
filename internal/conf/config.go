package conf

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"
)

var App struct {
	Port            int           `mapstructure:"port"`
	IPHeader        string        `mapstructure:"ip_header"`
	DrainDelay      time.Duration `mapstructure:"drain_delay"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

var Postgres struct {
	DSN string `mapstructure:"dsn"`
}

var Auth struct {
	// DisableSignUp stops new users from signing up, the existing users can still sign in.
	DisableSignUp bool `mapstructure:"disable_sign_up"`
	// SecretKey is the base64 of 32 bytes, which encrypts the secrets of the sign-in methods in the database.
	SecretKey string `mapstructure:"secret_key"`
}

var Redis struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

var Shortcut struct {
	// Workers is the number of the field shortcut jobs executed concurrently by the server, it defaults to 4.
	Workers int `mapstructure:"workers"`
}

// StorageConfig is the storage of the uploaded files.
type StorageConfig struct {
	// Type is the storage medium, local or s3, it defaults to local.
	Type string `mapstructure:"type"`
	// URLExpiry is the lifetime of the S3 presigned URLs, it defaults to 1 hour.
	URLExpiry time.Duration      `mapstructure:"url_expiry"`
	Local     LocalStorageConfig `mapstructure:"local"`
	S3        S3StorageConfig    `mapstructure:"s3"`
}

// LocalStorageConfig is the local disk storage.
type LocalStorageConfig struct {
	// Path is the root directory of the files, it defaults to ./data/storage.
	Path string `mapstructure:"path"`
}

// S3StorageConfig is the S3 or S3-compatible (MinIO, R2, COS, etc.) object storage.
type S3StorageConfig struct {
	// Endpoint uses the AWS endpoint if empty.
	Endpoint string `mapstructure:"endpoint"`
	Region   string `mapstructure:"region"`
	Bucket   string `mapstructure:"bucket"`
	// AccessKeyID uses the AWS default credential chain (environment variables, IAM role, etc.) if empty.
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UsePathStyle    bool   `mapstructure:"use_path_style"`
	// BasePath is the common prefix of all the object keys in the bucket.
	BasePath string `mapstructure:"base_path"`
	// PublicURL is the CDN or public-read bucket URL, the file URLs are built from it without presigning when set.
	PublicURL string `mapstructure:"public_url"`
}

var Storage StorageConfig

type TracingConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Token    string `mapstructure:"token"`
}

type MetricsConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Whitelist []string `mapstructure:"whitelist"`
}

var Observability struct {
	Tracing TracingConfig `mapstructure:"tracing"`
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// ShutdownGracePeriod is the budget for drain delay and graceful shutdown,
// excluding the reserve for interrupted job cleanup.
func ShutdownGracePeriod(timeout time.Duration) time.Duration {
	return timeout - min(5*time.Second, timeout/2)
}

func Init(configFilePath string) error {
	v := viper.New()
	v.SetDefault("app.drain_delay", "5s")
	v.SetDefault("app.shutdown_timeout", "30s")
	v.SetConfigFile(configFilePath)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrap(err, "read config file")
	}

	if err := v.UnmarshalKey("app", &App); err != nil {
		return errors.Wrap(err, "parse app")
	}

	App.DrainDelay = v.GetDuration("app.drain_delay")
	App.ShutdownTimeout = v.GetDuration("app.shutdown_timeout")
	if App.DrainDelay < 0 || App.ShutdownTimeout <= 0 || App.DrainDelay >= ShutdownGracePeriod(App.ShutdownTimeout) {
		return errors.New("app requires shutdown_timeout > 0 and 0 <= drain_delay < shutdown_timeout - min(5s, shutdown_timeout/2)")
	}

	if err := v.UnmarshalKey("postgres", &Postgres); err != nil {
		return errors.Wrap(err, "parse postgres")
	}
	if err := v.UnmarshalKey("auth", &Auth); err != nil {
		return errors.Wrap(err, "parse auth")
	}
	if err := v.UnmarshalKey("redis", &Redis); err != nil {
		return errors.Wrap(err, "parse redis")
	}
	if err := v.UnmarshalKey("observability", &Observability); err != nil {
		return errors.Wrap(err, "parse observability")
	}
	if err := v.UnmarshalKey("shortcut", &Shortcut); err != nil {
		return errors.Wrap(err, "parse shortcut")
	}
	if Shortcut.Workers <= 0 {
		Shortcut.Workers = 4
	}
	if err := v.UnmarshalKey("storage", &Storage); err != nil {
		return errors.Wrap(err, "parse storage")
	}
	if Storage.Type == "" {
		Storage.Type = "local"
	}
	if Storage.URLExpiry <= 0 {
		Storage.URLExpiry = time.Hour
	}
	if Storage.Local.Path == "" {
		Storage.Local.Path = "./data/storage"
	}

	return nil
}
