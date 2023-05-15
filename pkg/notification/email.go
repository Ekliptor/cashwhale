package notification

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// ensure we always implement Notifier (compile error otherwise)
var _ Notifier = (*Email)(nil)

type Email struct {
	config EmailConfig
}

type EmailConfig struct {
	// SMTP config of our mailbox for outgoing mail
	SmtpHost        string
	SmtpPort        int
	AllowSelfSigned bool
	FromAddress     string
	FromPassword    string

	RecAddress string // receiver. can be comma-separated list
}

// Address URI to smtp server.
func (c *EmailConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.SmtpHost, c.SmtpPort)
}

func NewEmail(config EmailConfig) (*Email, error) {
	if len(config.SmtpHost) < 4 || config.SmtpPort <= 0 {
		return nil, errors.New("invalid Email SMTP config")
	} else if len(config.FromAddress) < 5 || len(config.FromPassword) == 0 {
		return nil, errors.New("missing/invalid SMTP Email account to send mail")
	} else if len(config.RecAddress) < 5 {
		return nil, errors.New("receiver Email is missing")
	}
	return &Email{
		config: config,
	}, nil
}

func (e *Email) SendNotification(notification *Notification) error {
	notification.prepare()

	to := strings.Split(e.config.RecAddress, ",")
	for i, _ := range to {
		to[i] = strings.TrimSpace(to[i])
	}

	mailer := NewMailer(e.config.AllowSelfSigned)
	header := make(map[string]string)
	header["From"] = e.config.FromAddress
	header["To"] = to[0]
	header["Subject"] = mailer.EncodeRFC2047(notification.Title)
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	body := notification.Text
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	auth := smtp.PlainAuth("", e.config.FromAddress, e.config.FromPassword, e.config.SmtpHost)

	err := mailer.SendMail(e.config.Address(), auth, e.config.FromAddress, to, []byte(message))
	if err != nil {
		return errors.Wrap(err, "error sending Email")
	}

	return nil
}

type Mailer struct {
	localName              string // localhost
	dialTimeout            time.Duration
	allowSelfSignedTlsCert bool
}

func NewMailer(allowSelfSignedTlsCert bool) *Mailer {
	timeoutSec := viper.GetInt("Email.ConnectTimeoutSec")
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	return &Mailer{
		localName:              "localhost",
		dialTimeout:            time.Duration(timeoutSec) * time.Second,
		allowSelfSignedTlsCert: allowSelfSignedTlsCert,
	}
}

// Drop-in replacement for official net/smtp.SendMail() with:
// - a connect-timeout of 10 seconds
// - using implicit TLS instead of upgrading via STARTTLS
func (m *Mailer) SendMail(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: m.allowSelfSignedTlsCert,
	}
	if err = m.validateLine(from); err != nil {
		return err
	}

	if err := m.validateLine(from); err != nil {
		return err
	}
	for _, recp := range to {
		if err := m.validateLine(recp); err != nil {
			return err
		}
	}

	dialer := &net.Dialer{
		Timeout: m.dialTimeout,
	}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = c.Hello(m.localName); err != nil {
		return err
	}
	if err = c.Auth(auth); err != nil {
		return err
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

// Use the mail package RFC 2047 to encode any string
func (m *Mailer) EncodeRFC2047(str string) string {
	addr := mail.Address{Name: str, Address: ""}
	return strings.Trim(addr.String(), " <@>")
}

// validateLine checks to see if a line has CR or LF as per RFC 5321
func (m *Mailer) validateLine(line string) error {
	if strings.ContainsAny(line, "\n\r") {
		return errors.New("smtp: A line must not contain CR or LF")
	}
	return nil
}
