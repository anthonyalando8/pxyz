package emailhelper

import (
	"bytes"
	"context"
	"html/template"
	"log"

	emailpb "x/shared/genproto/emailpb"
	emailclient "x/shared/email"
)

// AdminEmailHelper helps with admin-related emails
type AdminEmailHelper struct {
	client *emailclient.EmailClient
}

// NewAdminEmailHelper constructor
func NewAdminEmailHelper(c *emailclient.EmailClient) *AdminEmailHelper {
	return &AdminEmailHelper{client: c}
}

// Email template for new admin account
const adminAccountCreatedTpl = `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Your Admin Account</title>
</head>
<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
  <div style="max-width: 600px; margin: auto; background: #fff; padding: 20px; border-radius: 8px;">
    <h2 style="color: #2E86C1;">Welcome to Admin Portal</h2>
    <p>Hello,</p>
    <p>Your admin account has been successfully created. Below are your credentials:</p>

    <p><strong>Email:</strong> {{.Email}}</p>
    <p><strong>Password:</strong> {{.Password}}</p>

    <p>Please login and change your password immediately.</p>
    <br>
    <p>Regards,<br><strong>Admin Team</strong></p>
  </div>
</body>
</html>
`

// Template data
type AdminAccountCreatedData struct {
	Email    string
	Password string
}

// SendAdminAccountCreated builds and sends an HTML email with admin credentials
func (h *AdminEmailHelper) SendAdminAccountCreated(ctx context.Context, userID, to, email, password string) {
	if h.client == nil || to == "" {
		return
	}

	go func(uid, recipient, email, password string) {
		tmpl, err := template.New("adminAccount").Parse(adminAccountCreatedTpl)
		if err != nil {
			log.Printf("[WARN] failed to parse email template: %v", err)
			return
		}

		var body bytes.Buffer
		if err := tmpl.Execute(&body, AdminAccountCreatedData{Email: email, Password: password}); err != nil {
			log.Printf("[WARN] failed to execute email template: %v", err)
			return
		}

		// Use context.Background() instead of ctx from request
		_, err = h.client.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         uid,
			RecipientEmail: recipient,
			Subject:        "Your Admin Account Has Been Created",
			Body:           body.String(),
			Type:           "admin_account_created",
		})
		if err != nil {
			log.Printf("[WARN] failed to send admin account created email to %s: %v", recipient, err)
		}
	}(userID, to, email, password)
}
