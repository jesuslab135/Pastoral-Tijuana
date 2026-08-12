package difusion

import (
	"math/rand/v2"
	"time"
)

// maxJitter spreads deliveries that would otherwise land on the same second,
// so a fanout does not read as a machine gun in a WhatsApp group.
const maxJitter = 3 * time.Second

// Stagger returns how long the nth delivery of one announcement waits. A base
// of zero means every delivery goes at once, which is what tests and a
// deliberately unthrottled deployment ask for.
func Stagger(n int, base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	jitter := time.Duration(rand.Int64N(int64(2*maxJitter))) - maxJitter
	d := time.Duration(n)*base + jitter
	if d < 0 {
		return 0
	}
	return d
}

// NextAllowed holds a message that would arrive during quiet hours until the
// window closes, judged on the parish's own clock rather than UTC. Equal
// bounds disable quiet hours; so does a value outside 0–23, which can only be
// a misconfiguration and must not park every message indefinitely.
func NextAllowed(t time.Time, loc *time.Location, quietStart, quietEnd int) time.Time {
	if quietStart == quietEnd || !validHour(quietStart) || !validHour(quietEnd) {
		return t
	}
	local := t.In(loc)
	h := local.Hour()

	quiet := h >= quietStart && h < quietEnd
	if quietStart > quietEnd {
		// The window crosses midnight, e.g. 22:00–07:00.
		quiet = h >= quietStart || h < quietEnd
	}
	if !quiet {
		return t
	}

	y, m, d := local.Date()
	release := time.Date(y, m, d, quietEnd, 0, 0, 0, loc)
	if h >= quietEnd {
		release = release.AddDate(0, 0, 1)
	}
	return release
}

func validHour(h int) bool { return h >= 0 && h <= 23 }
