package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/difusion"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

type broadcastJSONList struct {
	Broadcasts []struct {
		ID          string  `json:"id"`
		EventID     string  `json:"event_id"`
		ChannelID   string  `json:"channel_id"`
		ChannelName string  `json:"channel_name"`
		ChannelKind string  `json:"channel_kind"`
		Kind        string  `json:"kind"`
		State       string  `json:"state"`
		Attempt     int     `json:"attempt"`
		LastError   *string `json:"last_error"`
		Simulated   bool    `json:"simulated"`
	} `json:"broadcasts"`
}

func seedBroadcastFixture(t *testing.T, pool *pgxpool.Pool, kind string) (store.Broadcast, store.Event) {
	t.Helper()
	ctx := context.Background()
	ch := store.Channel{
		ID: uuid.New(), Kind: kind, Name: "canal-" + kind,
		Target: "destino", IsActive: true,
	}
	if err := store.CreateChannel(ctx, pool, ch); err != nil {
		t.Fatal(err)
	}
	e := store.Event{
		ID: uuid.New(), Title: "Hora santa", Place: "Templo",
		GroupID: uuid.MustParse(liturgiaGroup), Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
	}
	if err := store.CreateEvent(ctx, pool, e); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}
	var outboxID int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM outbox WHERE event_id = $1`, e.ID).Scan(&outboxID); err != nil {
		t.Fatal(err)
	}
	b := store.Broadcast{
		ID: uuid.New(), EventID: e.ID, ChannelID: ch.ID,
		Kind: store.OutboxPublished, State: "queued",
		DedupeKey: store.DedupeKey(e.ID, ch.ID, store.OutboxPublished, outboxID),
	}
	if _, err := store.CreateBroadcast(ctx, pool, b); err != nil {
		t.Fatal(err)
	}
	return b, e
}

func listBroadcasts(t *testing.T, r http.Handler, cookie *http.Cookie, query string) broadcastJSONList {
	t.Helper()
	rec := authed(t, r, cookie, "GET", "/api/v1/admin/broadcasts"+query, "")
	if rec.Code != 200 {
		t.Fatalf("list%s: expected 200, got %d: %s", query, rec.Code, rec.Body.String())
	}
	var body broadcastJSONList
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v (%s)", err, rec.Body.String())
	}
	return body
}

func TestListBroadcastsFiltersAndFlagsSimulated(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	wa, waEvent := seedBroadcastFixture(t, pool, "whatsapp")
	mail, _ := seedBroadcastFixture(t, pool, "email")
	if err := store.MarkBroadcastSent(ctx, pool, mail.ID); err != nil {
		t.Fatal(err)
	}

	all := listBroadcasts(t, r, cookie, "")
	if len(all.Broadcasts) != 2 {
		t.Fatalf("expected 2 broadcasts, got %d", len(all.Broadcasts))
	}
	for _, b := range all.Broadcasts {
		// v1 does not really send WhatsApp; the panel must say so.
		wantSimulated := b.ChannelKind == "whatsapp"
		if b.Simulated != wantSimulated {
			t.Errorf("%s channel: simulated=%v, want %v", b.ChannelKind, b.Simulated, wantSimulated)
		}
		if b.ChannelName == "" {
			t.Error("the panel shows the channel by name, not by id")
		}
	}

	sent := listBroadcasts(t, r, cookie, "?state=sent")
	if len(sent.Broadcasts) != 1 || sent.Broadcasts[0].ID != mail.ID.String() {
		t.Errorf("state filter: got %+v", sent.Broadcasts)
	}

	byEvent := listBroadcasts(t, r, cookie, "?event_id="+waEvent.ID.String())
	if len(byEvent.Broadcasts) != 1 || byEvent.Broadcasts[0].ID != wa.ID.String() {
		t.Errorf("event filter: got %+v", byEvent.Broadcasts)
	}
}

func TestListBroadcastsRejectsBadFilters(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	for _, q := range []string{"?state=inventado", "?event_id=no-es-uuid"} {
		if rec := authed(t, r, cookie, "GET", "/api/v1/admin/broadcasts"+q, ""); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d: %s", q, rec.Code, rec.Body.String())
		}
	}
}

func TestRetryBroadcastRequeuesAndEnqueues(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	rdb := testRedis(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	b, _ := seedBroadcastFixture(t, pool, "email")
	if err := store.MarkBroadcastFailed(ctx, pool, b.ID, "smtp caído", true); err != nil {
		t.Fatal(err)
	}

	inspector := asynq.NewInspectorFromRedisClient(rdb)
	t.Cleanup(func() { inspector.Close() })
	before := queueDepth(t, inspector, difusion.QueueMail)

	rec := authed(t, r, cookie, "POST", "/api/v1/admin/broadcasts/"+b.ID.String()+"/retry", "")
	if rec.Code != 200 {
		t.Fatalf("retry: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Broadcast struct {
			State string `json:"state"`
		} `json:"broadcast"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v (%s)", err, rec.Body.String())
	}
	if body.Broadcast.State != "queued" {
		t.Errorf("expected the row requeued, got %s", body.Broadcast.State)
	}
	if after := queueDepth(t, inspector, difusion.QueueMail); after != before+1 {
		t.Errorf("expected one more task on %s, went from %d to %d",
			difusion.QueueMail, before, after)
	}
}

func TestRetryRejectsWhatWasDelivered(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	b, _ := seedBroadcastFixture(t, pool, "email")
	if err := store.MarkBroadcastSent(ctx, pool, b.ID); err != nil {
		t.Fatal(err)
	}
	// Resending a delivered message would announce the same event twice.
	rec := authed(t, r, cookie, "POST", "/api/v1/admin/broadcasts/"+b.ID.String()+"/retry", "")
	if rec.Code != 409 {
		t.Errorf("retry on sent: expected 409, got %d: %s", rec.Code, rec.Body.String())
	}

	for _, id := range []string{uuid.New().String(), "no-es-uuid"} {
		rec := authed(t, r, cookie, "POST", "/api/v1/admin/broadcasts/"+id+"/retry", "")
		if rec.Code != 404 {
			t.Errorf("retry %s: expected 404, got %d", id, rec.Code)
		}
	}
}

func TestBroadcastsRequireASession(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/admin/broadcasts", nil))
	if rec.Code != 401 {
		t.Errorf("expected 401 without a cookie, got %d", rec.Code)
	}
}

func queueDepth(t *testing.T, in *asynq.Inspector, queue string) int {
	t.Helper()
	info, err := in.GetQueueInfo(queue)
	if err != nil {
		// A queue with nothing ever enqueued does not exist yet.
		return 0
	}
	return info.Size
}
