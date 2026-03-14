package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	conf "github.com/Servora-Kit/servora/api/gen/go/conf/v1"
	emailpkg "github.com/jordan-wright/email"
)

type smtpSender struct {
	host          string
	addr          string
	username      string
	password      string
	useTLS        bool
	skipVerifySSL bool
}

func newSMTPSender(c *conf.Smtp) *smtpSender {
	port := c.Port
	if port == 0 {
		port = 587
	}
	return &smtpSender{
		host:          c.Host,
		addr:          fmt.Sprintf("%s:%d", c.Host, port),
		username:      c.Username,
		password:      c.Password,
		useTLS:        c.Tls,
		skipVerifySSL: c.SkipVerifySsl,
	}
}

func (s *smtpSender) Send(_ context.Context, email Email) error {
	e := emailpkg.NewEmail()
	e.From = fmt.Sprintf("%s <%s>", email.From.Name, email.From.Address)
	e.To = email.To
	e.Cc = email.Cc
	e.Bcc = email.Bcc
	e.Subject = email.Subject
	e.Text = email.Text
	e.HTML = email.HTML

	var auth smtp.Auth
	if s.username != "" || s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if s.useTLS {
		return e.SendWithTLS(s.addr, auth, &tls.Config{
			ServerName:         s.host,
			InsecureSkipVerify: s.skipVerifySSL, //nolint:gosec // configurable for dev
		})
	}
	return e.Send(s.addr, auth)
}
