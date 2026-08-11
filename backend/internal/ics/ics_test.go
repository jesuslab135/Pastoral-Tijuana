package ics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
