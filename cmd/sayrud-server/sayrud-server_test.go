package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/route"
)

func TestServeDrainsReadinessAndFinishesHTTP(t *testing.T) {
	if os.Getenv("PGHOST") == "" || os.Getenv("REDIS_TEST_ADDRESS") == "" {
		t.Skip("PGHOST and REDIS_TEST_ADDRESS are required")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + randstr.Hex(12)
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		sqlDB, _ := admin.DB()
		_ = sqlDB.Close()
	})
	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := gormDB.DB()
		_ = sqlDB.Close()
	})
	require.NoError(t, gormDB.AutoMigrate(&db.SLShortcutJob{}))
	rdb := goredis.NewClient(&goredis.Options{Addr: os.Getenv("REDIS_TEST_ADDRESS")})
	t.Cleanup(func() { _ = rdb.Close() })
	runtime, err := route.NewRuntime(context.Background(), gormDB, rdb)
	require.NoError(t, err)
	signals, cancelSignals := context.WithCancel(context.Background())
	t.Cleanup(cancelSignals)
	requests, cancelRequests := context.WithCancel(context.Background())
	t.Cleanup(cancelRequests)
	started, release := make(chan struct{}), make(chan struct{})
	router := route.New(route.Options{DB: gormDB, Runtime: runtime})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			close(started)
			<-release
			_, _ = w.Write([]byte("finished"))
			return
		}
		router.ServeHTTP(w, r)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: handler, BaseContext: func(net.Listener) context.Context { return requests }}
	stopped := make(chan error, 1)
	go func() {
		stopped <- serve(signals, listener, server, runtime, cancelRequests, 200*time.Millisecond, 15*time.Second)
	}()
	base := "http://" + listener.Addr().String()
	response, err := http.Get(base + "/-/readyz")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	_ = response.Body.Close()
	finished := make(chan string, 1)
	go func() {
		response, err := http.Get(base + "/slow")
		if err != nil {
			finished <- err.Error()
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		finished <- string(body)
	}()
	<-started
	cancelSignals()
	require.Eventually(t, func() bool { return runtime.Ready(context.Background()) != nil }, time.Second, time.Millisecond)
	for _, path := range []string{"/-/readyz", "/-/healthz"} {
		response, err = http.Get(base + path)
		require.NoError(t, err)
		status := http.StatusOK
		if strings.Contains(path, "readyz") {
			status = http.StatusServiceUnavailable
		}
		require.Equal(t, status, response.StatusCode)
		_ = response.Body.Close()
	}
	close(release)
	require.Equal(t, "finished", <-finished)
	select {
	case err := <-stopped:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop")
	}
}
