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
			"MIME-Version: 1.0\r\n" + // required for HTML
			"Content-Type: text/html; charset=\"utf-8\"\r\n" +
			"\r\n" +
			body,
	)

	serverAddr := e.smtpHost + ":" + e.smtpPort

	// Implicit TLS for port 465
	tlsConfig := &tls.Config{
		ServerName: e.smtpHost,
	}

	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.smtpHost)
	if err != nil {
		return err
	}
	defer client.Quit()

	// Auth
	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
	if err := client.Auth(auth); err != nil {
		return err
	}

	// Set sender & recipient
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	// Write message
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

	return nil
}

// package service

// import (
// 	"crypto/tls"
// 	"fmt"
// 	"net"
// 	"net/smtp"
// )

// type EmailSender struct {
// 	smtpHost string
// 	smtpPort string
// 	username string
// 	password string
// }

// func NewEmailSender(host, port, user, pass string) *EmailSender {
// 	return &EmailSender{
// 		smtpHost: host,
// 		smtpPort: port,
// 		username: user,
// 		password: pass,
// 	}
// }

// func (e *EmailSender) Send(to, subject, body string) error {
// 	from := e.username
// 	msg := []byte(
// 		fmt.Sprintf("From: %s\r\n", from) +
// 			fmt.Sprintf("To: %s\r\n", to) +
// 			fmt.Sprintf("Subject: %s\r\n", subject) +
// 			"MIME-Version: 1.0\r\n" + // required for HTML
// 			"Content-Type: text/html; charset=\"utf-8\"\r\n" +
// 			"\r\n" +
// 			body,
// 	)

// 	serverAddr := e.smtpHost + ":" + e.smtpPort
// 	conn, err := net.Dial("tcp", serverAddr)
// 	if err != nil {
// 		return err
// 	}

// 	client, err := smtp.NewClient(conn, e.smtpHost)
// 	if err != nil {
// 		return err
// 	}

// 	tlsConfig := &tls.Config{ServerName: e.smtpHost}
// 	if err = client.StartTLS(tlsConfig); err != nil {
// 		return err
// 	}

// 	auth := smtp.PlainAuth("", e.username, e.password, e.smtpHost)
// 	if err := client.Auth(auth); err != nil {
// 		return err
// 	}

// 	if err := client.Mail(from); err != nil {
// 		return err
// 	}
// 	if err := client.Rcpt(to); err != nil {
// 		return err
// 	}

// 	w, err := client.Data()
// 	if err != nil {
// 		return err
// 	}
// 	if _, err := w.Write(msg); err != nil {
// 		return err
// 	}
// 	if err := w.Close(); err != nil {
// 		return err
// 	}

// 	return client.Quit()
// }

