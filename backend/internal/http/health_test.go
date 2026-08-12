package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestHealthz(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !body["ok"] {
		t.Errorf("expected ok:true, got %v", body)
	}
	if !body["redis"] {
		t.Errorf("test redis is up, so healthz must report redis:true, got %v", body)
	}
}

// TestHealthzRedisDown pins the split between the two dependencies: the API
// still serves the calendar without Redis (only magic links and difusión
// suffer), so a dead Redis must be reported, not turned into a 503.
func TestHealthzRedisDown(t *testing.T) {
	pool := testdb.New(t)
	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { dead.Close() })
	r := NewRouter(pool, dead, &mail.LogMailer{Sink: log.New(&bytes.Buffer{}, "", 0)}, config.Load())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != 200 {
		t.Fatalf("redis down is not fatal: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !body["ok"] || body["redis"] {
		t.Errorf("expected ok:true redis:false, got %v", body)
	}
}
