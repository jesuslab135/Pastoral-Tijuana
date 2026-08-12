package difusion

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
)

func TestStubWhatsAppSenderLogsSimulado(t *testing.T) {
	var buf bytes.Buffer
	s := &StubWhatsAppSender{Sink: log.New(&buf, "", 0)}

	err := s.Send(context.Background(), OutboundMessage{
		Target:  "120363000000000000@g.us",
		Subject: "Nuevo evento: Hora santa",
		Body:    "Hora santa\n📅 martes 4 de agosto, 12:00–13:00",
	})
	if err != nil {
		t.Fatalf("the stub always succeeds, got %v", err)
	}
	out := buf.String()
	// The panel labels these SIMULADO, and the log has to say the same thing
	// so nobody reads it as a message the parish actually received.
	for _, want := range []string{"SIMULADO", "120363000000000000@g.us", "Nuevo evento: Hora santa"} {
		if !strings.Contains(out, want) {
			t.Errorf("stub log must contain %q, got:\n%s", want, out)
		}
	}
}

func TestEmailSenderForwardsToMailer(t *testing.T) {
	var buf bytes.Buffer
	s := &EmailSender{Mailer: &mail.LogMailer{Sink: log.New(&buf, "", 0)}}

	err := s.Send(context.Background(), OutboundMessage{
		Target:  "avisos@parroquia.mx",
		Subject: "Evento cancelado: Hora santa",
		Body:    "Este evento se canceló.",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"avisos@parroquia.mx", "Evento cancelado: Hora santa", "Este evento se canceló."} {
		if !strings.Contains(out, want) {
			t.Errorf("mailer must receive %q, got:\n%s", want, out)
		}
	}
}

func TestSendersFromConfigCoversEveryChannelKind(t *testing.T) {
	senders := SendersFromConfig(config.Load())
	// channel_kind has exactly these two members; a channel with no sender
	// would fail every delivery at runtime instead of at boot.
	for _, kind := range []string{"whatsapp", "email"} {
		if senders[kind] == nil {
			t.Errorf("no sender configured for %s", kind)
		}
	}
}
