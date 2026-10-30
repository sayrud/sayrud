package setup

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCheckPostgresRejectsSoftDeletedUsers(t *testing.T) {
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}

	ctx := context.Background()
	admin, err := gorm.Open(postgres.New(postgres.Config{PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	name := "test_setup_" + randstr.Hex(12)
	require.NoError(t, admin.Exec("CREATE DATABASE "+name).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP DATABASE " + name).Error
		sqlDB, _ := admin.DB()
		_ = sqlDB.Close()
	})

	port, err := strconv.Atoi(os.Getenv("PGPORT"))
	require.NoError(t, err)
	opts := PostgresOptions{
		Host: os.Getenv("PGHOST"), Port: port, Database: name,
		User: os.Getenv("PGUSER"), Password: os.Getenv("PGPASSWORD"), SSLMode: "disable",
	}
	version, err := CheckPostgres(ctx, opts)
	require.NoError(t, err)
	assert.NotEmpty(t, version)

	database, err := gorm.Open(postgres.New(postgres.Config{DSN: opts.DSN(), PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	require.NoError(t, database.Exec("CREATE TABLE users (deleted_at timestamptz)").Error)
	_, err = CheckPostgres(ctx, opts)
	require.NoError(t, err, "an empty users table is usable")

	require.NoError(t, database.Exec("INSERT INTO users (deleted_at) VALUES (now())").Error)
	_, err = CheckPostgres(ctx, opts)
	require.ErrorIs(t, err, ErrDatabaseInUse)
}

func TestCheckRedisOptionalInfo(t *testing.T) {
	address := os.Getenv("REDIS_TEST_ADDRESS")
	if address == "" {
		t.Skip("REDIS_TEST_ADDRESS is not set")
	}

	ctx := context.Background()
	admin := redis.NewClient(&redis.Options{Addr: address})
	defer func() { _ = admin.Close() }()

	version, err := CheckRedis(ctx, RedisOptions{Address: address})
	require.NoError(t, err)
	assert.NotEmpty(t, version)

	username := "test_setup_" + randstr.Hex(12)
	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", username, "on", ">test-password", "+ping", "+hello", "+client").Err())
	t.Cleanup(func() { _ = admin.Do(ctx, "ACL", "DELUSER", username).Err() })

	opts := RedisOptions{Address: address, Username: username, Password: "test-password"}
	version, err = CheckRedis(ctx, opts)
	require.NoError(t, err, "INFO permission is not needed")
	assert.Empty(t, version)

	require.NoError(t, admin.Do(ctx, "ACL", "SETUSER", username, "-ping").Err())
	_, err = CheckRedis(ctx, opts)
	require.Error(t, err, "PING failure still fails the check")
}
