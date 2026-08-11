package httpapi

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
)

// testRedis connects to TEST_REDIS_ADDR, defaulting to the dev compose
// address. When the variable is set explicitly (CI does), an unreachable
// Redis is a failure: skipping there would quietly green-light untested
// magic-link code. Only a local run with no variable set may skip.
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr, explicit := os.LookupEnv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(context.Background()).Err(); err != nil {
		c.Close()
		if explicit {
			t.Fatalf("redis no disponible en %s: %v", addr, err)
		}
		t.Skipf("redis no disponible en %s (define TEST_REDIS_ADDR para exigirlo): %v", addr, err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func newRouter(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	r, _ := newRouterWithMail(t, pool)
	return r
}

// newRouterWithMail also returns the buffer the log mailer writes to, so a
// test can read back the magic link that was "sent".
func newRouterWithMail(t *testing.T, pool *pgxpool.Pool) (http.Handler, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	mailer := &mail.LogMailer{Sink: log.New(buf, "", 0)}
	return NewRouter(pool, testRedis(t), mailer, config.Load()), buf
}
