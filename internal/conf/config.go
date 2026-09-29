package conf

import (
	"github.com/cockroachdb/errors"
	"github.com/spf13/viper"
)

var App struct {
	Port int `mapstructure:"port"`
}

var Postgres struct {
	DSN string `mapstructure:"dsn"`
}

var Redis struct {
	Address  string `mapstructure:"address"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

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

func Init(configFilePath string) error {
	v := viper.New()
	v.SetConfigFile(configFilePath)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrap(err, "read config file")
	}

	if err := v.UnmarshalKey("app", &App); err != nil {
		return errors.Wrap(err, "parse app")
	}
	if err := v.UnmarshalKey("postgres", &Postgres); err != nil {
		return errors.Wrap(err, "parse postgres")
	}
	if err := v.UnmarshalKey("redis", &Redis); err != nil {
		return errors.Wrap(err, "parse redis")
	}
	if err := v.UnmarshalKey("observability", &Observability); err != nil {
		return errors.Wrap(err, "parse observability")
	}

	return nil
}
