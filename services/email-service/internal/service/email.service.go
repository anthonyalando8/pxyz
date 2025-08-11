package service

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
)

type EmailSender struct {
	smtpHost string
	smtpPort string
	username string
	password string
}

func NewEmailSender(host, port, user, pass string) *EmailSender {
	return &EmailSender{
		smtpHost: host,
		smtpPort: port,
		username: user,
		password: pass,
	}
}

func (e *EmailSender) Send(to, subject, body string) error {
	from := e.username
	msg := []byte(
		fmt.Sprintf("From: %s\r\n", from) +
			fmt.Sprintf("To: %s\r\n", to) +
			fmt.Sprintf("Subject: %s\r\n", subject) +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
			"\r\n" +
			body,
	)

	// Step 1: Connect over TCP (no TLS yet)
	serverAddr := e.smtpHost + ":" + e.smtpPort
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return err
	}

	// Step 2: Create SMTP client
	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return err
	}

	// Step 3: Start TLS
	tlsConfig := &tls.Config{
		ServerName: e.smtpHost,
	}
	if err = client.StartTLS(tlsConfig); err != nil {
		return err
	}

	// Step 4: Authenticate
	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	if err := client.Auth(auth); err != nil {
		return err
	}

	// Step 5: Send mail
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}
