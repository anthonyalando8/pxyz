package handler

import (
	//"context"
	"context"
	"fmt"
	"log"

	emailclient "x/shared/email"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"


	"x/shared/genproto/emailpb"

)

// Optional notification stubs
func sendPartnerUpdatedNotification(_ context.Context, partnerUserID string) {
	fmt.Printf("Partner user updated: %s\n", partnerUserID)
}

func sendPartnerDeletedNotification(_ context.Context, partnerUserID string) {
	fmt.Printf("Partner user deleted: %s\n", partnerUserID)
}

func sendNewPartnerUserNotifications(_ context.Context, uc *usecase.PartnerUsecase, emailClient *emailclient.EmailClient,partnerID string, partnerEmail string, user *domain.PartnerUser, password string) {
	if emailClient == nil {
		return
	}

	// Send notification to Partner
	go func(pid, recipient string, user *domain.PartnerUser) {
		// If partnerEmail is empty, fetch it
		if recipient == "" {
			partner, err := uc.GetPartnerByID(context.Background(), pid)
			if err != nil {
				log.Printf("[WARN] failed to fetch partner %s for notification: %v", pid, err)
				return
			}
			recipient = partner.ContactEmail
		}

		if recipient == "" {
			log.Printf("[WARN] partner %s has no contact email; skipping notification", pid)
			return
		}

		subject := "New Partner User Created"
		body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html><head><meta charset="UTF-8"><title>New Partner User</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">New User Added</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						A new user has been added to your partner account <strong>%s</strong>.<br>
						User ID: <strong>%s</strong><br>
						Email: <strong>%s</strong>
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>`, pid, user.ID, user.Email)

		_, err := emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         user.ID,
			RecipientEmail: recipient,
			Subject:        subject,
			Body:           body,
			Type:           "partner_new_user",
		})
		if err != nil {
			log.Printf("[WARN] failed to send partner notification to %s: %v", recipient, err)
		}
	}(partnerID, partnerEmail, user)

	// Send notification to User
	if user.Email != "" {
		go func(user *domain.PartnerUser, pwd, pid string) {
			subject := "Your Pxyz Partner Account Details"
			body := fmt.Sprintf(`
				<!DOCTYPE html>
				<html><head><meta charset="UTF-8"><title>Welcome</title></head>
				<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
					<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
						<h2 style="color: #2E86C1;">Welcome to Pxyz</h2>
						<p style="font-size: 16px; color: #333;">
							Hello,<br><br>
							Your partner user account has been created.<br>
							Partner ID: <strong>%s</strong><br>
							User ID: <strong>%s</strong><br>
							Email: <strong>%s</strong><br>
							Password: <strong>%s</strong>
						</p>
						<p style="margin-top: 30px; font-size: 14px; color: #999999;">
							You may log in and change your password.<br>
							Thank you,<br>
							<strong>Pxyz Team</strong>
						</p>
					</div>
				</body>
				</html>`, pid, user.ID, user.Email, pwd)

			_, err := emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         user.ID,
				RecipientEmail: user.Email,
				Subject:        subject,
				Body:           body,
				Type:           "user_welcome",
			})
			if err != nil {
				log.Printf("[WARN] failed to send user notification to %s: %v", user.Email, err)
			}
		}(user, password, partnerID)
	}
}


func sendPartnerCreatedEmail(
	_ *usecase.PartnerUsecase,
	emailClient *emailclient.EmailClient,
	p *domain.Partner,
	adminPassword string,
) {
	if emailClient == nil || p.ContactEmail == "" {
		return
	}

	go func(partner *domain.Partner, password string) {
		subject := "Your Partner Account Has Been Created"
		body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html><head><meta charset="UTF-8"><title>Partner Created</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px;
							border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">Welcome to Pxyz</h2>
					
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						Your partner account has been successfully created.<br><br>
						<strong>Partner Name:</strong> %s <br>
						<strong>Partner ID:</strong> %s
					</p>

					<p style="font-size: 16px; color: #333; margin-top:20px;">
						We’ve also created a default <strong>Partner Admin</strong> account for you.<br>
						You can access the Partner Dashboard using the following credentials:
					</p>

					<div style="background:#f4f6f8; padding:12px; border-radius:6px; margin-top:10px;">
						<p style="font-size: 15px; color:#333; margin:0;">
							<strong>Email:</strong> %s <br>
							<strong>Password:</strong> %s
						</p>
					</div>

					<p style="font-size: 14px; color:#cc0000; margin-top:15px;">
						⚠️ For security reasons, please log in immediately and change your password.
					</p>

					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>`, 
			partner.Name, partner.ID, partner.ContactEmail, password)

		_, err := emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         partner.ID,
			RecipientEmail: partner.ContactEmail,
			Subject:        subject,
			Body:           body,
			Type:           "partner_created",
		})
		if err != nil {
			log.Printf("[WARN] failed to send partner notification to %s: %v", partner.ContactEmail, err)
		}
	}(p, adminPassword)
}
