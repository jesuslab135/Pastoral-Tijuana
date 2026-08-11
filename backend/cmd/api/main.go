package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	httpapi "github.com/jesuslab135/pastoral-tijuana/backend/internal/http"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

func main() {
	cfg := config.Load()

	sqldb, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(sqldb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	sqldb.Close()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	log.Printf("pastoral api listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, httpapi.NewRouter(pool, cfg)); err != nil {
		log.Fatal(err)
	}
}
