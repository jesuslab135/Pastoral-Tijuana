package difusion

import (
	"strings"
	"testing"
	"time"

	// Embedded so these tests do not depend on the host's zone database.
	_ "time/tzdata"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const baseURL = "https://pastoral.example.mx"

func tijuana(t *testing.T) *time.Location { return parishTZ(t, "America/Tijuana") }

func parishTZ(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load parish timezone %s: %v", name, err)
	}
	return loc
}

func samplePayload() store.OutboxPayload {
	return store.OutboxPayload{
		ID:          uuid.New(),
		Title:       "Hora santa",
		Description: "Adoración al Santísimo",
		Place:       "Templo parroquial",
		// Tuesday 19:00Z: 12:00 in Tijuana (-07:00 in August), 13:00 in
		// Mexico City (-06:00 all year since 2022).
		StartsAt: time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		Rank:     "parroquial",
	}
}

func TestRenderSubjectsPerKind(t *testing.T) {
	loc := tijuana(t)
	p := samplePayload()

	for kind, want := range map[store.OutboxKind]string{
		store.OutboxPublished: "Nuevo evento: Hora santa",
		store.OutboxUpdated:   "Cambio de horario o lugar: Hora santa",
		store.OutboxCancelled: "Evento cancelado: Hora santa",
	} {
		subject, body := Render(kind, p, loc, baseURL)
		if subject != want {
			t.Errorf("%s subject: got %q, want %q", kind, subject, want)
		}
		if !strings.Contains(body, "Hora santa") {
			t.Errorf("%s body must name the event:\n%s", kind, body)
		}
		if !strings.Contains(body, "Ver calendario: "+baseURL) {
			t.Errorf("%s body must link the calendar:\n%s", kind, body)
		}
	}
}

// TestRenderUsesParishLocalTimeInSpanish pins that the hour a parishioner
// reads is the hour in the parish, whichever timezone PARISH_TZ names: an
// event stored as 19:00Z must never surface as 19:00.
func TestRenderUsesParishLocalTimeInSpanish(t *testing.T) {
	for tz, want := range map[string]struct{ start, end string }{
		"America/Tijuana":     {"12:00", "13:00"},
		"America/Mexico_City": {"13:00", "14:00"},
	} {
		body := renderBody(t, store.OutboxPublished, samplePayload(), parishTZ(t, tz))
		if !strings.Contains(body, "martes 4 de agosto") {
			t.Errorf("%s: expected the Spanish weekday and month:\n%s", tz, body)
		}
		if !strings.Contains(body, want.start) || !strings.Contains(body, want.end) {
			t.Errorf("%s: times must be parish-local (%s–%s):\n%s", tz, want.start, want.end, body)
		}
		if strings.Contains(body, "19:00") {
			t.Errorf("%s: UTC time leaked into the message:\n%s", tz, body)
		}
	}
}

func TestRenderIncludesPlaceAndDescriptionOnlyWhenSet(t *testing.T) {
	loc := tijuana(t)
	full := renderBody(t, store.OutboxPublished, samplePayload(), loc)
	if !strings.Contains(full, "📍 Templo parroquial") {
		t.Errorf("place missing:\n%s", full)
	}
	if !strings.Contains(full, "Adoración al Santísimo") {
		t.Errorf("description missing:\n%s", full)
	}

	bare := samplePayload()
	bare.Place = ""
	bare.Description = ""
	body := renderBody(t, store.OutboxPublished, bare, loc)
	if strings.Contains(body, "📍") {
		t.Errorf("an empty place must not render a pin:\n%s", body)
	}
	// No blank line where the description would have been.
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			t.Errorf("no empty lines when optional fields are absent:\n%q", body)
			break
		}
	}
}

func TestRenderCancelledLeadsWithTheRetraction(t *testing.T) {
	loc := tijuana(t)
	body := renderBody(t, store.OutboxCancelled, samplePayload(), loc)
	first, _, _ := strings.Cut(body, "\n")
	if first != "Este evento se canceló." {
		t.Errorf("a cancellation must say so first, got %q", first)
	}
}

func TestRenderSpanishWeekdaysAndMonths(t *testing.T) {
	loc := tijuana(t)
	for _, tc := range []struct {
		day  time.Time
		want string
	}{
		// Local dates, so the rendered day cannot be shifted by the offset.
		{time.Date(2026, 1, 4, 10, 0, 0, 0, loc), "domingo 4 de enero"},
		{time.Date(2026, 3, 16, 10, 0, 0, 0, loc), "lunes 16 de marzo"},
		{time.Date(2026, 12, 25, 10, 0, 0, 0, loc), "viernes 25 de diciembre"},
	} {
		p := samplePayload()
		p.StartsAt = tc.day
		p.EndsAt = tc.day.Add(time.Hour)
		body := renderBody(t, store.OutboxPublished, p, loc)
		if !strings.Contains(body, tc.want) {
			t.Errorf("expected %q in:\n%s", tc.want, body)
		}
	}
}

func renderBody(t *testing.T, kind store.OutboxKind, p store.OutboxPayload, loc *time.Location) string {
	t.Helper()
	_, body := Render(kind, p, loc, baseURL)
	return body
}
