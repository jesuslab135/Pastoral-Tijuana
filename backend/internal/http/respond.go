// Package httpapi wires the chi router, handlers, and JSON helpers.
package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writePublicJSON answers one of the public read endpoints. The body carries
// an ETag and is marked must-revalidate rather than fresh for a fixed window.
// The panel's whole promise is that what the parish publishes or cancels shows
// up on the site; a plain `max-age` left a stale calendar in the browser with
// no way for it to ask whether anything had moved, so an event deleted in the
// panel kept rendering on the site for minutes after it was gone from the
// database — and read as a deletion that had not worked. Revalidating costs
// one conditional request and answers 304 for as long as nothing changes.
func writePublicJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal",
			"No se pudo generar la respuesta.")
		return
	}
	// Over the body rather than over max(updated_at) as the .ics feed does:
	// these payloads change when a row is deleted outright, which no surviving
	// row's timestamp records.
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// etagMatches reports whether an If-None-Match header covers etag. The
// comparison the header calls for is the weak one, so a tag a cache echoed
// back marked `W/` still counts.
func etagMatches(header, etag string) bool {
	if strings.TrimSpace(header) == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == etag {
			return true
		}
	}
	return false
}

type errBody struct {
	Error errDetail `json:"error"`
}
type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errBody{Error: errDetail{Code: code, Message: msg}})
}
