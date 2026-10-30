package setup

import (
	"context"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// checkTimeout bounds each connection test, so that an unreachable host does not hold the wizard.
const checkTimeout = 5 * time.Second

// ErrDatabaseInUse is returned when the PostgreSQL database already has the accounts of another installation.
var ErrDatabaseInUse = errors.New("database already has users")

// CheckError is a failed connection test of a service.
type CheckError struct {
	// Service is either "postgres" or "redis".
	Service string
	Err     error
}

func (e *CheckError) Error() string {
	return e.Service + ": " + e.Err.Error()
}

func (e *CheckError) Unwrap() error {
	return e.Err
}

// CheckPostgres connects to PostgreSQL and returns the server version.
// It returns ErrDatabaseInUse if the database already has users, so that the wizard never takes over an existing installation.
func CheckPostgres(ctx context.Context, opts PostgresOptions) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: opts.DSN(), PreferSimpleProtocol: true}), &gorm.Config{
		Logger:               logger.Discard,
		DisableAutomaticPing: true,
	})
	if err != nil {
		return "", errors.Wrap(err, "open")
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return "", errors.Wrap(err, "get connection")
	}
	defer func() { _ = sqlDB.Close() }()

	if err := sqlDB.PingContext(ctx); err != nil {
		return "", err
	}
	var version string
	if err := gormDB.WithContext(ctx).Raw("SHOW server_version").Scan(&version).Error; err != nil {
		return "", errors.Wrap(err, "get version")
	}

	var hasUsers bool
	if err := gormDB.WithContext(ctx).Raw(`SELECT to_regclass('users') IS NOT NULL`).Scan(&hasUsers).Error; err != nil {
		return "", errors.Wrap(err, "check users table")
	}
	if hasUsers {
		if err := gormDB.WithContext(ctx).Raw(`SELECT EXISTS (SELECT 1 FROM users WHERE deleted_at IS NULL)`).Scan(&hasUsers).Error; err != nil {
			return "", errors.Wrap(err, "check users")
		}
	}
	if hasUsers {
		return version, ErrDatabaseInUse
	}
	return version, nil
}

// CheckRedis connects to Redis and returns the server version.
func CheckRedis(ctx context.Context, opts RedisOptions) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr:                  opts.Address,
		Username:              opts.Username,
		Password:              opts.Password,
		ContextTimeoutEnabled: true,
		MaxRetries:            -1,
	})
	defer func() { _ = client.Close() }()

	if err := client.Ping(ctx).Err(); err != nil {
		return "", err
	}
	info, err := client.Info(ctx, "server").Result()
	if err != nil {
		return "", errors.Wrap(err, "get info")
	}
	for _, line := range strings.Split(info, "\n") {
		if version, ok := strings.CutPrefix(strings.TrimSpace(line), "redis_version:"); ok {
			return version, nil
		}
	}
	return "", nil
}

// CheckResult is the result of testing a service, Err is nil if it is usable.
type CheckResult struct {
	Version string
	Err     error
}

// CheckServices tests PostgreSQL and Redis concurrently.
func CheckServices(ctx context.Context, opts DatabaseOptions) (pg, rdb CheckResult) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		rdb.Version, rdb.Err = CheckRedis(ctx, opts.Redis)
	}()
	pg.Version, pg.Err = CheckPostgres(ctx, opts.Postgres)
	<-done
	return pg, rdb
}
