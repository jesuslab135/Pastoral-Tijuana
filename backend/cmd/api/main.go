package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	// Embeds the timezone database so PARISH_TZ resolves in images that carry
	// no /usr/share/zoneinfo (scratch, alpine without tzdata).
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	httpapi "github.com/jesuslab135/pastoral-tijuana/backend/internal/http"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const (
	readHeaderTimeout = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 20 * time.Second
)

func main() {
	cfg := config.Load()

	if _, err := time.LoadLocation(cfg.ParishTZ); err != nil {
		log.Fatalf("PARISH_TZ: %v", err)
	}
	if _, err := cfg.PublicHost(); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sqldb, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := store.MigrateLocked(ctx, sqldb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	sqldb.Close()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(pool, cfg),
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("pastoral api listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		log.Fatalf("serve: %v", err)
	case <-ctx.Done():
		log.Print("shutting down, draining in-flight requests")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
