package redis

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
)

var _ AuthAttemptsStore = (*authAttempts)(nil)

// AuthAttempts is the default instance of the AuthAttemptsStore.
var AuthAttempts AuthAttemptsStore

// AuthAttemptsStore limits the attempts of signing up, signing in and verifying passwords.
type AuthAttemptsStore interface {
	// CheckSignUp counts a sign-up attempt of the IP.
	// It returns ErrTooManyAttempts if the IP has signed up too many times recently.
	CheckSignUp(ctx context.Context, ip string) error
	// CheckSignIn counts a sign-in attempt of the IP and the email.
	// It returns ErrTooManyAttempts if either of them has tried too many times recently.
	CheckSignIn(ctx context.Context, ip, email string) error
	// CheckPassword counts an attempt of verifying the password of the email, sharing the limit with signing in.
	// It returns ErrTooManyAttempts if the email has tried too many times recently.
	CheckPassword(ctx context.Context, email string) error
	// ResetSignIn clears the attempts of the email after signing in successfully.
	ResetSignIn(ctx context.Context, email string) error
}

func NewAuthAttemptsStore(client *redis.Client) AuthAttemptsStore {
	return &authAttempts{client}
}

type authAttempts struct {
	// Client is the Redis connection the store operates on.
	*redis.Client
}

var ErrTooManyAttempts = errors.New("too many attempts")

const (
	signUpLimit  = 10
	signUpWindow = time.Hour

	signInIPLimit    = 30
	signInEmailLimit = 10
	signInWindow     = 10 * time.Minute
)

func signUpKey(ip string) string         { return "auth:sign_up:" + ip }
func signInIPKey(ip string) string       { return "auth:sign_in:ip:" + ip }
func signInEmailKey(email string) string { return "auth:sign_in:email:" + email }

func (db *authAttempts) CheckSignUp(ctx context.Context, ip string) error {
	return db.count(ctx, signUpKey(ip), signUpLimit, signUpWindow)
}

func (db *authAttempts) CheckSignIn(ctx context.Context, ip, email string) error {
	if err := db.count(ctx, signInIPKey(ip), signInIPLimit, signInWindow); err != nil {
		return err
	}
	return db.count(ctx, signInEmailKey(email), signInEmailLimit, signInWindow)
}

func (db *authAttempts) CheckPassword(ctx context.Context, email string) error {
	return db.count(ctx, signInEmailKey(email), signInEmailLimit, signInWindow)
}

func (db *authAttempts) ResetSignIn(ctx context.Context, email string) error {
	return db.Del(ctx, signInEmailKey(email)).Err()
}

// count increases the counter of the key in a fixed window, and returns ErrTooManyAttempts once it exceeds max.
func (db *authAttempts) count(ctx context.Context, key string, max int64, window time.Duration) error {
	n, err := db.Incr(ctx, key).Result()
	if err != nil {
		return errors.Wrap(err, "incr")
	}
	if n == 1 {
		if err := db.Expire(ctx, key, window).Err(); err != nil {
			return errors.Wrap(err, "expire")
		}
	}
	if n > max {
		return ErrTooManyAttempts
	}
	return nil
}
