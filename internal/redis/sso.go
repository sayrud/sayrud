package redis

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
)

var _ SSOStatesStore = (*ssoStates)(nil)

// SSOStates is the default instance of the SSOStatesStore.
var SSOStates SSOStatesStore

// SSOStatesStore keeps the one-time states of third-party sign-in and the replay marks of SAML assertions.
type SSOStatesStore interface {
	// Save saves the sign-in context of the state.
	Save(ctx context.Context, state string, value []byte, ttl time.Duration) error
	// Take returns and deletes the sign-in context of the state, it returns ErrSSOStateNotFound if the state does not exist or has expired.
	Take(ctx context.Context, state string) ([]byte, error)
	// MarkOnce marks the key until it expires, it returns false if the key has been marked.
	MarkOnce(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

func NewSSOStatesStore(client *redis.Client) SSOStatesStore {
	return &ssoStates{client}
}

type ssoStates struct {
	*redis.Client
}

var ErrSSOStateNotFound = errors.New("sso state does not exist")

func ssoStateKey(state string) string { return "sso:state:" + state }
func ssoOnceKey(key string) string    { return "sso:once:" + key }

func (db *ssoStates) Save(ctx context.Context, state string, value []byte, ttl time.Duration) error {
	return db.Set(ctx, ssoStateKey(state), value, ttl).Err()
}

func (db *ssoStates) Take(ctx context.Context, state string) ([]byte, error) {
	if state == "" {
		return nil, ErrSSOStateNotFound
	}
	value, err := db.GetDel(ctx, ssoStateKey(state)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSSOStateNotFound
		}
		return nil, err
	}
	return value, nil
}

func (db *ssoStates) MarkOnce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl < time.Second {
		ttl = time.Second
	}
	return db.SetNX(ctx, ssoOnceKey(key), 1, ttl).Result()
}
