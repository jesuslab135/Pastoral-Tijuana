// Package ics renders RFC 5545 iCalendar feeds for phone subscriptions.
package ics

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID
	Title       string
	Place       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	UpdatedAt   time.Time
	Cancelled   bool
}

const stamp = "20060102T150405Z"

// escape implements RFC 5545 §3.3.11 TEXT escaping.
func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", "")
	return r.Replace(s)
}

// fold breaks a content line into 75-octet chunks with continuation lines.
func fold(line string) []string {
	const limit = 75
	b := []byte(line)
	if len(b) <= limit {
		return []string{line}
	}
	var out []string
	cur := limit
	// Never split inside a UTF-8 rune.
	for cur < len(b) && b[cur]&0xC0 == 0x80 {
		cur--
	}
	out = append(out, string(b[:cur]))
	rest := " " + string(b[cur:])
	out = append(out, fold(rest)...)
	return out
}

func Build(calName, host string, events []Event) string {
	var lines []string
	add := func(l string) { lines = append(lines, fold(l)...) }

	add("BEGIN:VCALENDAR")
	add("VERSION:2.0")
	add("PRODID:-//Calendario Pastoral//" + host + "//ES")
	add("CALSCALE:GREGORIAN")
	add("METHOD:PUBLISH")
	add("X-WR-CALNAME:" + escape(calName))

	for _, e := range events {
		add("BEGIN:VEVENT")
		add("UID:" + e.ID.String() + "@" + host)
		add("DTSTAMP:" + e.UpdatedAt.UTC().Format(stamp))
		add("DTSTART:" + e.StartsAt.UTC().Format(stamp))
		add("DTEND:" + e.EndsAt.UTC().Format(stamp))
		add("SUMMARY:" + escape(e.Title))
		if e.Place != "" {
			add("LOCATION:" + escape(e.Place))
		}
		if e.Description != "" {
			add("DESCRIPTION:" + escape(e.Description))
		}
		add("SEQUENCE:" + itoa(e.UpdatedAt.Unix()))
		if e.Cancelled {
			add("STATUS:CANCELLED")
		} else {
			add("STATUS:CONFIRMED")
		}
		add("END:VEVENT")
	}
	add("END:VCALENDAR")
	return strings.Join(lines, "\r\n") + "\r\n"
}

func itoa(n int64) string {
	if n < 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	return string(b[i:])
}
