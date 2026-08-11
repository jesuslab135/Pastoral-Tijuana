package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "db_unavailable",
				"La base de datos no responde.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
