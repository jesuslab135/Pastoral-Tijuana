// Package difusion turns published events into messages and delivers them to
// the parish's WhatsApp groups and mailing list.
package difusion

import (
	"strconv"
	"strings"
	"time"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

// Go's standard library formats dates in English only, so the parish's own
// words live here.
var (
	weekdays = [...]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	months   = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio",
		"julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
)

var subjects = map[store.OutboxKind]string{
	store.OutboxPublished: "Nuevo evento: ",
	store.OutboxUpdated:   "Cambio de horario o lugar: ",
	store.OutboxCancelled: "Evento cancelado: ",
}

// Render produces the Spanish subject and plain-text body for one broadcast.
// It reads the outbox snapshot rather than the live event, so an edit made
// while a message is queued cannot rewrite what was already announced.
func Render(kind store.OutboxKind, p store.OutboxPayload, loc *time.Location, publicBaseURL string) (string, string) {
	subject := subjects[kind] + p.Title

	var lines []string
	if kind == store.OutboxCancelled {
		lines = append(lines, "Este evento se canceló.")
	}
	lines = append(lines, p.Title, "📅 "+formatWhen(p.StartsAt, p.EndsAt, loc))
	if p.Place != "" {
		lines = append(lines, "📍 "+p.Place)
	}
	if p.Description != "" {
		lines = append(lines, p.Description)
	}
	lines = append(lines, "Ver calendario: "+publicBaseURL)
	return subject, strings.Join(lines, "\n")
}

// formatWhen writes the moment as the parish reads it: local time, Spanish
// weekday and month, and only the clock time for the end when it is the same
// day, which it almost always is.
func formatWhen(starts, ends time.Time, loc *time.Location) string {
	s := starts.In(loc)
	e := ends.In(loc)
	out := spanishDate(s) + ", " + s.Format("15:04")
	if sameDay(s, e) {
		return out + "–" + e.Format("15:04")
	}
	return out + " – " + spanishDate(e) + ", " + e.Format("15:04")
}

func spanishDate(t time.Time) string {
	return weekdays[int(t.Weekday())] + " " +
		strconv.Itoa(t.Day()) + " de " + months[int(t.Month())-1]
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
