package service

import (
	"crypto/tls"
	"fmt"
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

	// TLS connection
	serverAddr := e.smtpHost + ":" + e.smtpPort
	conn, err := tls.Dial("tcp", serverAddr, &tls.Config{
		InsecureSkipVerify: true, // for testing only
		ServerName:         e.smtpHost,
	})
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return err
	}

	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	if err := client.Auth(auth); err != nil {
		return err
	}

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
