// Package notify sends the run log as a plain-text email over SMTP (with
// optional STARTTLS and LOGIN/PLAIN auth) using only the Go standard library.
// The HTML/ANSI-to-HTML path of the Python original was intentionally dropped.
package notify

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// Message is a plain-text email.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
}

// Sender delivers a Message. The interface makes the app testable with a mock.
type Sender interface {
	Send(msg Message) error
}

// SMTPSender sends mail via an SMTP server.
type SMTPSender struct {
	Host     string
	Port     int
	TLS      bool // use STARTTLS
	Username string
	Password string
}

// Send connects to the SMTP server and delivers msg.
func (s *SMTPSender) Send(msg Message) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	defer c.Close()

	// EHLO with a stable name (matches the Python ehlo name usage loosely).
	if err := c.Hello(ehloName(s.Host)); err != nil {
		return fmt.Errorf("smtp hello: %w", err)
	}

	if s.TLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if s.Username != "" && s.Password != "" {
		auth := chooseAuth(c, s.Username, s.Password, s.Host)
		if auth != nil {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err := c.Mail(msg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(buildRFC822(msg))); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

func buildRFC822(msg Message) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", msg.From)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	// Normalise to CRLF line endings.
	body := strings.ReplaceAll(msg.Body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	b.WriteString(body)
	return b.String()
}

func ehloName(host string) string {
	if host == "" {
		return "localhost"
	}
	return host
}

// chooseAuth prefers PLAIN when advertised, otherwise falls back to LOGIN
// (which net/smtp does not provide out of the box). Mirrors the Python
// "LOGIN PLAIN" capability handling.
func chooseAuth(c *smtp.Client, user, pass, host string) smtp.Auth {
	if ok, mechs := c.Extension("AUTH"); ok {
		if strings.Contains(mechs, "PLAIN") {
			return smtp.PlainAuth("", user, pass, host)
		}
		if strings.Contains(mechs, "LOGIN") {
			return &loginAuth{username: user, password: pass}
		}
	}
	// No AUTH advertised; attempt PLAIN anyway (server may still accept).
	return smtp.PlainAuth("", user, pass, host)
}
