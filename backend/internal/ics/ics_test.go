package ics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func fixedEvents() []Event {
	return []Event{
		{
			ID:          uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			Title:       "Misa solemne de la Asunción",
			Place:       "Templo parroquial",
			Description: "Misa solemne; el horario ordinario cambia.",
			StartsAt:    time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC),
			EndsAt:      time.Date(2026, 8, 15, 19, 30, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			Title:     "Junta de pastoral; sala 2",
			StartsAt:  time.Date(2026, 8, 12, 1, 30, 0, 0, time.UTC),
			EndsAt:    time.Date(2026, 8, 12, 3, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
			Cancelled: true,
		},
	}
}

func TestBuildGolden(t *testing.T) {
	got := Build("Calendario Pastoral · Cristo de Los Álamos",
		"app.jesuslab135.com", fixedEvents())

	golden := filepath.Join("testdata", "calendar.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run once with UPDATE_GOLDEN=1): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden file.\nGot:\n%s", got)
	}
}

func TestBuildProperties(t *testing.T) {
	got := Build("Cal", "app.jesuslab135.com", fixedEvents())
	for _, want := range []string{
		"BEGIN:VCALENDAR", "END:VCALENDAR",
		"UID:11111111-1111-4111-8111-111111111111@app.jesuslab135.com",
		"DTSTART:20260815T180000Z",
		"STATUS:CANCELLED",
		"SEQUENCE:", "METHOD:PUBLISH",
		"SUMMARY:Junta de pastoral\\; sala 2", // semicolon escaped
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(got, "\n") && !strings.Contains(got, "\r\n") {
		t.Error("lines must end with CRLF")
	}
	for _, line := range strings.Split(got, "\r\n") {
		if len([]byte(line)) > 75 {
			t.Errorf("line exceeds 75 octets: %q", line)
		}
	}
}

// TestFoldLongMultibyteLines is a regression test for fold(): it guards the
// 75-octet folding invariants (no line over 75 octets, no UTF-8 rune ever
// split across a fold boundary, no bytes lost or duplicated) using SUMMARY
// and DESCRIPTION content long enough, and at enough different byte
// alignments, that the fold boundary is guaranteed to land inside a
// multi-byte rune in at least one case. Nothing in the golden fixture is
// long enough to exercise fold()'s split path, so without this test that
// path is dead code under CI.
func TestFoldLongMultibyteLines(t *testing.T) {
	const (
		titleMix = "áé·日本語" // 2-byte, 2-byte, 2-byte, 3-byte, 3-byte, 3-byte runes
		descMix  = "ñÑ€中文测"  // 2-byte, 2-byte, 3-byte, 3-byte, 3-byte, 3-byte runes
	)
	knownPrefixes := []string{
		"BEGIN:", "END:", "VERSION:", "PRODID:", "CALSCALE:", "METHOD:",
		"X-WR-CALNAME:", "UID:", "DTSTAMP:", "DTSTART:", "DTEND:",
		"SUMMARY:", "LOCATION:", "DESCRIPTION:", "SEQUENCE:", "STATUS:",
	}

	for i := 0; i < 8; i++ {
		pad := strings.Repeat("x", i)
		title := pad + strings.Repeat(titleMix, 15)
		desc := pad + strings.Repeat(descMix, 15)
		if n := len([]byte(title)); n < 150 {
			t.Fatalf("pad=%d: title fixture too short: %d octets", i, n)
		}

		events := []Event{{
			ID:          uuid.MustParse("33333333-3333-4333-8333-333333333333"),
			Title:       title,
			Description: desc,
			StartsAt:    time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC),
			EndsAt:      time.Date(2026, 8, 15, 19, 30, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		}}
		got := Build("Cal", "app.jesuslab135.com", events)

		// Assertion 4: the whole output must be valid UTF-8 -- this is what
		// actually catches a rune split by fold().
		if !utf8.ValidString(got) {
			t.Fatalf("pad=%d: output is not valid UTF-8 (a multi-byte rune was split)", i)
		}

		lines := strings.Split(got, "\r\n")
		sawContinuation := false
		for _, line := range lines {
			if line == "" {
				continue // trailing empty element after the final CRLF
			}
			// Assertion 1: no line exceeds 75 octets.
			if n := len([]byte(line)); n > 75 {
				t.Errorf("pad=%d: line exceeds 75 octets (len=%d): %q", i, n, line)
			}
			// Assertion 2: every line is either a continuation (starts with
			// exactly one space) or the first line of a known property --
			// i.e. the first line of a folded property never starts with a
			// space.
			if strings.HasPrefix(line, " ") {
				sawContinuation = true
				continue
			}
			hasKnownPrefix := false
			for _, p := range knownPrefixes {
				if strings.HasPrefix(line, p) {
					hasKnownPrefix = true
					break
				}
			}
			if !hasKnownPrefix {
				t.Errorf("pad=%d: line is neither a continuation nor a recognized property start: %q", i, line)
			}
		}
		if !sawContinuation {
			t.Errorf("pad=%d: expected folding to occur (no continuation line found) for a %d-octet title", i, len([]byte(title)))
		}

		// Assertion 3: round trip -- stripping the CRLF+space continuation
		// sequences must reproduce the original title and description
		// exactly once each, proving fold() drops or duplicates no bytes.
		unfolded := strings.ReplaceAll(got, "\r\n ", "")
		if c := strings.Count(unfolded, title); c != 1 {
			t.Errorf("pad=%d: unfolded output contains original title %d times, want 1", i, c)
		}
		if c := strings.Count(unfolded, desc); c != 1 {
			t.Errorf("pad=%d: unfolded output contains original description %d times, want 1", i, c)
		}
	}
}
