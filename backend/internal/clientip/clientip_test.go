package clientip

import (
	"net/http/httptest"
	"testing"
)

func TestUntrustedPeerIgnoresXFF(t *testing.T) {
	r, err := NewResolver("")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:9999"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 198.51.100.2")
	if got := r.FromRequest(req); got != "203.0.113.7" {
		t.Errorf("untrusted peer: got %q, want socket ip", got)
	}
}

func TestTrustedProxyUsesLastXFFHop(t *testing.T) {
	r, err := NewResolver("172.18.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:4321" // Caddy's container address
	// The client forged the first hop; Caddy appended the real one last.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.2")
	if got := r.FromRequest(req); got != "198.51.100.2" {
		t.Errorf("trusted proxy: got %q, want last XFF hop", got)
	}
}

func TestTrustedProxyNoXFFFallsBack(t *testing.T) {
	r, _ := NewResolver("172.18.0.0/16")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:4321"
	if got := r.FromRequest(req); got != "172.18.0.5" {
		t.Errorf("no XFF: got %q, want socket ip", got)
	}
}

func TestTrustedProxyGarbageXFFFallsBack(t *testing.T) {
	r, _ := NewResolver("172.18.0.0/16")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:4321"
	req.Header.Set("X-Forwarded-For", "no-soy-una-ip")
	if got := r.FromRequest(req); got != "172.18.0.5" {
		t.Errorf("garbage XFF: got %q, want socket ip", got)
	}
}

func TestBadCIDRRejected(t *testing.T) {
	if _, err := NewResolver("not-a-cidr"); err == nil {
		t.Error("invalid CIDR must be rejected at construction")
	}
}
