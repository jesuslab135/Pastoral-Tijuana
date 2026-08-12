package difusion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

// deactivatedChannel is the reason shown in the panel when a channel was
// switched off after its message was queued.
const deactivatedChannel = "canal desactivado"

// Deliver renders one broadcast from its outbox snapshot, sends it through
// the channel's sender and settles the row either way.
//
// It separates two kinds of "cannot send". A row that no longer applies —
// missing, already delivered, addressed to a channel that is gone or switched
// off — is skipped: retrying it would never succeed, and asynq would keep the
// task alive for nothing. A send that failed is reported back, so asynq
// retries it until the budget runs out and the row turns dead.
func Deliver(ctx context.Context, pool *pgxpool.Pool, senders map[string]Sender,
	loc *time.Location, publicBaseURL string, p DeliverPayload, retried, maxRetry int) error {
	b, err := store.GetBroadcast(ctx, pool, p.BroadcastID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("difusion: broadcast %s ya no existe, se omite", p.BroadcastID)
			return nil
		}
		return err
	}
	if b.State == "sent" {
		// A duplicated queue entry: the parish already got this one.
		return nil
	}

	ch, err := store.GetChannel(ctx, pool, b.ChannelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("difusion: canal %s ya no existe, se omite", b.ChannelID)
			return store.MarkBroadcastFailed(ctx, pool, b.ID, "canal eliminado", true)
		}
		return err
	}
	if !ch.IsActive {
		return store.MarkBroadcastFailed(ctx, pool, b.ID, deactivatedChannel, true)
	}

	row, err := store.GetOutboxRow(ctx, pool, p.OutboxID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("difusion: outbox %d ya no existe, se omite", p.OutboxID)
			return store.MarkBroadcastFailed(ctx, pool, b.ID, "aviso original eliminado", true)
		}
		return err
	}

	sender, ok := senders[ch.Kind]
	if !ok {
		// Configuration, not transport: fail it so it shows up rather than
		// disappearing into a silent skip.
		err := fmt.Errorf("sin remitente para canales de tipo %q", ch.Kind)
		return settleFailure(ctx, pool, b.ID, err, retried, maxRetry)
	}

	subject, body := Render(row.Kind, row.Payload, loc, publicBaseURL)
	if err := sender.Send(ctx, OutboundMessage{
		Target: ch.Target, Subject: subject, Body: body,
	}); err != nil {
		return settleFailure(ctx, pool, b.ID, err, retried, maxRetry)
	}
	return store.MarkBroadcastSent(ctx, pool, b.ID)
}

// settleFailure records the attempt and hands the error back to asynq, which
// decides whether there is another try left.
func settleFailure(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, cause error, retried, maxRetry int) error {
	dead := retried >= maxRetry
	if markErr := store.MarkBroadcastFailed(ctx, pool, id, cause.Error(), dead); markErr != nil {
		return errors.Join(cause, markErr)
	}
	return cause
}
