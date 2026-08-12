package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// redisPingTimeout keeps a hung Redis from stalling the health check, which
// load balancers poll on a short interval.
const redisPingTimeout = time.Second

// healthHandler answers 503 only for Postgres: without it nothing works. A
// dead Redis costs magic links and difusión but still serves the calendar,
// so it is reported as a flag rather than taken as a reason to fail over.
func healthHandler(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "db_unavailable",
				"La base de datos no responde.")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), redisPingTimeout)
		defer cancel()
		redisOK := rdb != nil && rdb.Ping(ctx).Err() == nil
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "redis": redisOK})
	}
}
