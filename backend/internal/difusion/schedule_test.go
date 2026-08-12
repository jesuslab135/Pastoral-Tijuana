package difusion

import (
	"testing"
	"time"

	_ "time/tzdata"
)

func TestNextAllowed(t *testing.T) {
	loc := parishTZ(t, "America/Tijuana")
	at := func(day, hour, min int) time.Time {
		return time.Date(2026, 8, day, hour, min, 0, 0, loc)
	}

	for name, tc := range map[string]struct {
		now        time.Time
		start, end int
		want       time.Time
	}{
		"late night waits for morning": {at(4, 23, 30), 22, 7, at(5, 7, 0)},
		"small hours wait same day":    {at(4, 3, 0), 22, 7, at(4, 7, 0)},
		"midday goes out now":          {at(4, 12, 0), 22, 7, at(4, 12, 0)},
		"quiet starts on the hour":     {at(4, 22, 0), 22, 7, at(5, 7, 0)},
		"quiet ends on the hour":       {at(4, 7, 0), 22, 7, at(4, 7, 0)},
		"disabled sends at any hour":   {at(4, 23, 30), 7, 7, at(4, 23, 30)},
		"same-day window holds":        {at(4, 14, 0), 13, 15, at(4, 15, 0)},
		"same-day window outside":      {at(4, 16, 0), 13, 15, at(4, 16, 0)},
		"same-day window before":       {at(4, 12, 59), 13, 15, at(4, 12, 59)},
	} {
		got := NextAllowed(tc.now, loc, tc.start, tc.end)
		if !got.Equal(tc.want) {
			t.Errorf("%s: NextAllowed(%s, %d, %d) = %s, want %s",
				name, tc.now.Format(time.RFC3339), tc.start, tc.end,
				got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
		}
	}
}

// TestNextAllowedKeepsTheParishClock guards the case a UTC-only
// implementation gets wrong: 23:30 local is 06:30 UTC the next day, so
// deciding quiet hours on the wall clock of the wrong zone sends at midnight.
func TestNextAllowedJudgesInParishTime(t *testing.T) {
	loc := parishTZ(t, "America/Tijuana")
	utcLateNight := time.Date(2026, 8, 5, 6, 30, 0, 0, time.UTC) // 23:30 in Tijuana
	got := NextAllowed(utcLateNight, loc, 22, 7)
	want := time.Date(2026, 8, 5, 7, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestStagger(t *testing.T) {
	const base = 8 * time.Second
	for n := 0; n < 5; n++ {
		lo := time.Duration(n)*base - maxJitter
		if lo < 0 {
			lo = 0
		}
		hi := time.Duration(n)*base + maxJitter
		// Sampled: the jitter is random, so one call proves nothing.
		for i := 0; i < 50; i++ {
			d := Stagger(n, base)
			if d < 0 {
				t.Fatalf("Stagger(%d) returned a negative delay %s", n, d)
			}
			if d < lo || d > hi {
				t.Fatalf("Stagger(%d) = %s, want within [%s, %s]", n, d, lo, hi)
			}
		}
	}
}

func TestStaggerZeroBaseIsImmediate(t *testing.T) {
	// The end-to-end test turns stagger off; that must mean "send now", not
	// "send within three seconds".
	for i := 0; i < 20; i++ {
		if d := Stagger(3, 0); d != 0 {
			t.Fatalf("with base 0 every delivery is immediate, got %s", d)
		}
	}
}
