package mail

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

func TestNewSelectsLogMailerWithoutSMTP(t *testing.T) {
	if m := New(config.Config{}); !isLog(m) {
		t.Fatalf("expected LogMailer, got %T", m)
	}
}

func TestNewSelectsSMTPMailerWithHost(t *testing.T) {
	m := New(config.Config{
		SMTPHost: "smtp.ionos.mx", SMTPPort: "587",
		SMTPUser: "u", SMTPPass: "p", SMTPFrom: "cal@parroquia.mx",
	})
	if _, ok := m.(*SMTPMailer); !ok {
		t.Fatalf("expected SMTPMailer, got %T", m)
	}
}

func isLog(m Mailer) bool {
	_, ok := m.(*LogMailer)
	return ok
}

func TestLogMailerLogsEverything(t *testing.T) {
	var buf bytes.Buffer
	m := &LogMailer{Sink: log.New(&buf, "", 0)}
	if err := m.Send(context.Background(), "p@x.mx", "Tu enlace",
		"https://pastoral.jesuslab135.com/verify?token=abc"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"p@x.mx", "Tu enlace", "token=abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q:\n%s", want, out)
		}
	}
}
