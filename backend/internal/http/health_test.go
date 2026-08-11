package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

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
}
