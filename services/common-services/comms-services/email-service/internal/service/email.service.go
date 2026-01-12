// internal/service/email_sender.go
package service

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EmailSender struct {
	smtpHost    string
	smtpPort    string
	username    string
	password    string
	fromName    string  // Display name
	replyTo     string  // Reply-to address
	domainName  string  // Your domain for Message-ID
}

type EmailConfig struct {
	SMTPHost   string
	SMTPPort   string
	Username   string
	Password   string
	FromName   string  // e.g., "SafariGari Support"
	ReplyTo    string  // e.g., "support@safarigari.com"
	DomainName string  // e.g., "safarigari.com"
}

func NewEmailSender(config EmailConfig) *EmailSender {
	return &EmailSender{
		smtpHost:   config.SMTPHost,
		smtpPort:   config. SMTPPort,
		username:    config.Username,
		password:   config.Password,
		fromName:   config.FromName,
		replyTo:    config.ReplyTo,
		domainName: config.DomainName,
	}
}

type EmailMessage struct {
	To          string
	Subject     string
	HTMLBody    string
	PlainBody   string  // ✅ Always include plain text version
	Category    string  // e.g., "transactional", "notification"
}

func (e *EmailSender) Send(msg EmailMessage) error {
	// Generate Message-ID (required for deliverability)
	messageID := fmt.Sprintf("<%s@%s>", uuid.New().String(), e.domainName)
	
	// Build proper From header with display name
	from := fmt.Sprintf("%s <%s>", e.fromName, e.username)
	
	// Get current date in RFC2822 format
	date := time.Now().Format(time.RFC1123Z)
	
	// Build multipart email with both HTML and plain text
	boundary := "----=_Part_" + uuid.New().String()
	
	// Build headers
	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", msg. To),
		fmt.Sprintf("Reply-To: %s", e.replyTo),
		fmt.Sprintf("Subject: %s", msg.Subject),
		fmt.Sprintf("Date: %s", date),
		fmt.Sprintf("Message-ID: %s", messageID),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"", boundary),
		"X-Mailer: SafariGari-Mailer/1.0",  // Identify your mailer
		"X-Priority: 3",  // Normal priority
		"X-MSMail-Priority:  Normal",
		"Importance: Normal",
	}
	
	// Add category header (useful for tracking)
	if msg.Category != "" {
		headers = append(headers, fmt.Sprintf("X-Email-Category: %s", msg. Category))
	}
	
	// Build email body
	body := []string{
		"",  // Empty line after headers
		"This is a multi-part message in MIME format. ",
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 7bit",
		"",
		msg.PlainBody,
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 7bit",
		"",
		msg.HTMLBody,
		"",
		fmt.Sprintf("--%s--", boundary),
	}
	
	// Combine headers and body
	fullMessage := strings.Join(headers, "\r\n") + "\r\n" + strings.Join(body, "\r\n")
	
	return e.sendRaw(msg.To, []byte(fullMessage))
}

func (e *EmailSender) sendRaw(to string, message []byte) error {
	serverAddr := e.smtpHost + ":" + e.smtpPort

	// Implicit TLS for port 465
	tlsConfig := &tls.Config{
		ServerName:          e.smtpHost,
		InsecureSkipVerify: false,  // Always verify certificates
		MinVersion:         tls.VersionTLS12,  // Use modern TLS
	}

	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client, err := smtp. NewClient(conn, e.smtpHost)
	if err != nil {
		return fmt. Errorf("failed to create SMTP client: %w", err)
	}
	defer client. Quit()

	// Auth
	auth := smtp.PlainAuth("", e.username, e. password, e.smtpHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set sender & recipient
	if err := client. Mail(e.username); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Write message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data:  %w", err)
	}
	
	if _, err := w.Write(message); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data: %w", err)
	}

	return nil
}

// ✅ Helper to convert HTML to plain text (basic version)
func HTMLToPlainText(html string) string {
	// Remove HTML tags
	text := html
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = strings.ReplaceAll(text, "</p>", "\n\n")
	text = strings.ReplaceAll(text, "</div>", "\n")
	
	// Remove remaining HTML tags
	for strings.Contains(text, "<") && strings.Contains(text, ">") {
		start := strings.Index(text, "<")
		end := strings.Index(text, ">")
		if start < end {
			text = text[: start] + text[end+1:]
		} else {
			break
		}
	}
	
	// Clean up whitespace
	text = strings.TrimSpace(text)
	
	return text
}