package difusion

import (
	"context"
	"log"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
)

// OutboundMessage is one rendered message addressed to one channel.
type OutboundMessage struct {
	Target  string // WhatsApp group JID or email address
	Subject string
	Body    string
}

// Sender is the seam a real WhatsApp provider drops into later: v1 ships a
// stub, and swapping it changes nothing else in the engine.
type Sender interface {
	Send(ctx context.Context, msg OutboundMessage) error
}

// StubWhatsAppSender records what would have been sent and always succeeds.
// Broadcasts through it show in the panel as SIMULADO, so the parish is never
// told a message went out that did not.
type StubWhatsAppSender struct{ Sink *log.Logger }

func (s *StubWhatsAppSender) Send(_ context.Context, msg OutboundMessage) error {
	sink := s.Sink
	if sink == nil {
		sink = log.Default()
	}
	sink.Printf("WHATSAPP SIMULADO\nPara: %s\nAsunto: %s\n\n%s\n",
		msg.Target, msg.Subject, msg.Body)
	return nil
}

// EmailSender adapts the transactional mailer, which sends over SMTP when one
// is configured and logs otherwise.
type EmailSender struct{ Mailer mail.Mailer }

func (s *EmailSender) Send(ctx context.Context, msg OutboundMessage) error {
	return s.Mailer.Send(ctx, msg.Target, msg.Subject, msg.Body)
}

// SendersFromConfig maps every channel kind to its sender. The keys match the
// channel_kind enum: a missing one would only surface as failed deliveries.
func SendersFromConfig(cfg config.Config) map[string]Sender {
	return map[string]Sender{
		"whatsapp": &StubWhatsAppSender{},
		"email":    &EmailSender{Mailer: mail.New(cfg)},
	}
}
