// Package mail delivers transactional email. Plan 3's difusión SMTPSender
// reuses the same SMTP_* configuration but is a separate component.
package mail

import (
	"context"
	"fmt"
	"log"
	"net/smtp"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, textBody string) error
}

// New picks the SMTP mailer when a host is configured and the logging mailer
// otherwise, so development needs no mail server.
func New(cfg config.Config) Mailer {
	if cfg.SMTPHost == "" {
		return &LogMailer{}
	}
	return &SMTPMailer{
		addr: cfg.SMTPHost + ":" + cfg.SMTPPort,
		auth: smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost),
		from: cfg.SMTPFrom,
	}
}

// LogMailer prints the message instead of sending it, so magic links are
// copy-pasteable from the console in development.
type LogMailer struct{ Sink *log.Logger }

func (m *LogMailer) Send(_ context.Context, to, subject, textBody string) error {
	sink := m.Sink
	if sink == nil {
		sink = log.Default()
	}
	sink.Printf("MAIL (sin SMTP configurado)\nTo: %s\nSubject: %s\n\n%s\n", to, subject, textBody)
	return nil
}

type SMTPMailer struct {
	addr string
	auth smtp.Auth
	from string
}

func (m *SMTPMailer) Send(_ context.Context, to, subject, textBody string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		m.from, to, subject, textBody)
	// smtp.SendMail negotiates STARTTLS when the server advertises it.
	return smtp.SendMail(m.addr, m.auth, m.from, []string{to}, []byte(msg))
}
