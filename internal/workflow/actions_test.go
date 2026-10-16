package workflow

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestAllowRateExpiresCounter(t *testing.T) {
	address := os.Getenv("REDIS_TEST_ADDRESS")
	if address == "" {
		t.Skip("REDIS_TEST_ADDRESS is not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	engine := NewEngine(nil, nil, rdb, 1)
	ip := randstr.String(12)
	rl := RateLimit{Enabled: true, Limit: 2, Window: 60}

	for i, want := range []bool{true, true, false} {
		allowed, _ := engine.AllowRate(ctx, 1, ip, rl)
		assert.Equal(t, want, allowed, "request %d", i+1)
	}
	keys, err := rdb.Keys(ctx, "api_rate:1:"+ip+":*").Result()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), keys...).Err() })
	ttl, err := rdb.TTL(ctx, keys[0]).Result()
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= time.Minute, "the counter expires with the window, got %v", ttl)
}

func TestSendSMTPAddressPolicy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request internal-banner\r\n"))
			_ = conn.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	ctx := context.Background()
	m := Mail{To: []string{"ann@example.com"}}

	for _, host := range []string{"127.0.0.1", "localhost"} {
		err := SendSMTP(ctx, SMTPServer{Host: host, Port: p, Security: MailSecurityNone}, m, nil)
		var actionErr *ActionError
		require.ErrorAs(t, err, &actionErr, host)
		assert.Contains(t, actionErr.Message, "is not public", host)
		assert.False(t, actionErr.Transient, "a blocked address is not retried")
	}

	networks, err := db.ParseNetworkAllowlist([]string{"127.0.0.1"})
	require.NoError(t, err)
	err = SendSMTP(ctx, SMTPServer{Host: "127.0.0.1", Port: p, Security: MailSecurityNone}, m, networks)
	var actionErr *ActionError
	require.ErrorAs(t, err, &actionErr)
	assert.Contains(t, actionErr.Message, "SMTP greeting", "the allowlisted address is connected")
	assert.NotContains(t, actionErr.Message, "internal-banner", "the response of a non-SMTP service is not echoed")
}
