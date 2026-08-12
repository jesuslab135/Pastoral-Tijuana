package main

import (
	"context"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"

	// Embeds the timezone database so PARISH_TZ resolves in images that carry
	// no /usr/share/zoneinfo (scratch, alpine without tzdata).
	_ "time/tzdata"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/difusion"
)

const (
	// waConcurrency is 1 on purpose: WhatsApp providers throttle, and a
	// parish group flooded in one second reads as spam.
	waConcurrency   = 1
	mailConcurrency = 4
)

func main() {
	cfg := config.Load()

	loc, err := time.LoadLocation(cfg.ParishTZ)
	if err != nil {
		log.Fatalf("PARISH_TZ: %v", err)
	}
	// Messages carry the calendar link, so a bad base URL would go out to the
	// whole parish before anyone noticed.
	if _, err := cfg.PublicHost(); err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// The API owns migrations. Saying so explicitly beats a worker that
	// starts against an empty database and fails one query at a time.
	var schemaVersion int64
	if err := pool.QueryRow(ctx,
		`SELECT max(version_id) FROM goose_db_version`).Scan(&schemaVersion); err != nil {
		log.Fatalf("la base de datos no está migrada (la migra cmd/api): %v", err)
	}
	log.Printf("pastoral worker: esquema en la versión %d", schemaVersion)

	redisOpt := asynq.RedisClientOpt{Addr: cfg.RedisAddr}
	client := asynq.NewClient(redisOpt)
	defer client.Close()

	mux := difusion.NewMux(pool, client, difusion.SendersFromConfig(cfg), loc, cfg)

	// Two servers because the queues have opposite needs: WhatsApp must stay
	// serialized while mail and fanout run wide.
	srvWA := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: waConcurrency,
		Queues:      map[string]int{difusion.QueueWA: 1},
	})
	srvMail := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: mailConcurrency,
		Queues:      map[string]int{difusion.QueueMail: 1, difusion.QueueFanout: 1},
	})

	for name, srv := range map[string]*asynq.Server{"wa": srvWA, "mail": srvMail} {
		if err := srv.Start(mux); err != nil {
			log.Fatalf("start %s server: %v", name, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		difusion.RunRelay(ctx, pool, client)
	}()

	log.Print("pastoral worker escuchando avisos")
	<-ctx.Done()
	log.Print("apagando, se esperan los envíos en curso")

	wg.Wait()
	// Shutdown waits for in-flight handlers, so a message being sent is never
	// cut in half.
	srvWA.Shutdown()
	srvMail.Shutdown()
}
