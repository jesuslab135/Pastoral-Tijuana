// Package clientip resolves the real client address behind an optional
// trusted reverse proxy. chi's RealIP middleware was removed on purpose: it
// trusts X-Forwarded-For unconditionally, and Caddy appends to that header,
// so a client can prepend a forged address. Nothing else in the app may read
// X-Forwarded-For directly.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type Resolver struct {
	trusted *netip.Prefix // nil = trust nobody
}

// NewResolver builds a resolver. An empty CIDR trusts no proxy, which is the
// correct default for a directly exposed server and for local development.
func NewResolver(trustedProxyCIDR string) (*Resolver, error) {
	if trustedProxyCIDR == "" {
		return &Resolver{}, nil
	}
	p, err := netip.ParsePrefix(trustedProxyCIDR)
	if err != nil {
		return nil, err
	}
	return &Resolver{trusted: &p}, nil
}

// FromRequest returns the last X-Forwarded-For hop only when the socket peer
// is inside the trusted CIDR (that hop is the one the proxy itself wrote);
// otherwise it returns the socket peer address.
func (r *Resolver) FromRequest(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if r.trusted == nil {
		return host
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !r.trusted.Contains(peer) {
		return host
	}
	xff := req.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	hops := strings.Split(xff, ",")
	last := strings.TrimSpace(hops[len(hops)-1])
	if _, err := netip.ParseAddr(last); err != nil {
		return host
	}
	return last
}
