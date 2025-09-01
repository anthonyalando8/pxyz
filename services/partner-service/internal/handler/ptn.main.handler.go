package handler

import (
	//"context"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	accountclient "x/shared/account"
	emailclient "x/shared/email"
	smsclient "x/shared/sms"
	"x/shared/utils/id"

	domain "partner-service/internal/domain"
	"partner-service/internal/usecase"
	authclient "x/shared/auth" // gRPC/HTTP client for auth-service
	otpclient "x/shared/auth/otp"
	"x/shared/response"

	"x/shared/genproto/authpb"
	"x/shared/genproto/emailpb"
)

type PartnerHandler struct {
	uc         *usecase.PartnerUsecase
	authClient *authclient.AuthService
	otp *otpclient.OTPService
	accountClient *accountclient.AccountClient
	emailClient *emailclient.EmailClient
	smsClient *smsclient.SMSClient
}

func NewPartnerHandler(
	uc *usecase.PartnerUsecase,
	authClient *authclient.AuthService,
	otp          *otpclient.OTPService,
	accountClient *accountclient.AccountClient,
	emailClient *emailclient.EmailClient,
	smsClient *smsclient.SMSClient,
) *PartnerHandler {
	return &PartnerHandler{
		uc:         uc,
		authClient: authClient,
		otp:            otp,
		accountClient:  accountClient,
		emailClient:    emailClient,
		smsClient:      smsClient,
	}
}

// ----------- Handlers -----------
func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
// CreatePartner
func (h *PartnerHandler) CreatePartner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Name         string `json:"name"`
		Country      string `json:"country"`
		ContactEmail string `json:"contact_email"`
		ContactPhone string `json:"contact_phone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	// var role = domain.PartnerUserRoleAdmin
	// password, err := id.GeneratePassword()
	// if err != nil {
	// 	response.Error(w, http.StatusInternalServerError, "failed to generate password: "+err.Error())
	// 	return
	// }
	

	// // Step 1: Create user in auth service
	// userResp, err := h.authClient.RegisterUser(ctx, &authpb.RegisterUserRequest{
	// 	Email:     req.Email,
	// 	Password:  req.Password,
	// 	FirstName: req.FirstName,
	// 	LastName:  req.LastName,
	// 	Role:      string(role), // send plain string to auth
	// })
	// if err != nil {
	// 	response.Error(w, http.StatusInternalServerError, "failed to create user in auth service: "+err.Error(),)
	// 	return
	// }

	partner := &domain.Partner{
		ID:           id.GenerateID("PTN"),
		Name:         req.Name,
		Country:      req.Country,
		ContactEmail: req.ContactEmail,
		ContactPhone: req.ContactPhone,
	}

	if err := h.uc.CreatePartner(ctx, partner); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Send partner notification using helper
	h.sendPartnerCreatedEmail(partner)

	response.JSON(w, http.StatusCreated, partner)
}

func (h *PartnerHandler) sendPartnerCreatedEmail(p *domain.Partner) {
	if h.emailClient == nil || p.ContactEmail == "" {
		return
	}

	go func(partner *domain.Partner) {
		subject := "Your Partner Account Has Been Created"
		body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html><head><meta charset="UTF-8"><title>Partner Created</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">Welcome to Pxyz</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						Your partner account has been successfully created.<br>
						Partner Name: <strong>%s</strong><br>
						Partner ID: <strong>%s</strong>
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>`, partner.Name, partner.ID)

		_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         partner.ID,
			RecipientEmail: partner.ContactEmail,
			Subject:        subject,
			Body:           body,
			Type:           "partner_created",
		})
		if err != nil {
			log.Printf("[WARN] failed to send partner notification to %s: %v", partner.ContactEmail, err)
		}
	}(p)
}


// CreatePartnerUser (calls auth service to create user first)
func (h *PartnerHandler) CreatePartnerUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		PartnerID string `json:"partner_id"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Role      string `json:"role"` // incoming as plain string
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest,err.Error(),)
		return
	}

	// validate role
	var role domain.PartnerUserRole
	switch req.Role {
	case string(domain.PartnerUserRoleAdmin):
		role = domain.PartnerUserRoleAdmin
	case string(domain.PartnerUserRoleUser):
		role = domain.PartnerUserRoleUser
	default:
		response.Error(w,http.StatusBadRequest, "invalid role, must be 'partner_admin' or 'partner_user'", )
		return
	}
	if req.Password == ""{
		var err error
		req.Password, err = id.GeneratePassword()
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "failed to generate password: "+err.Error())
			return
		}
	}

	// Step 1: Create user in auth service
	userResp, err := h.authClient.RegisterUser(ctx, &authpb.RegisterUserRequest{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      string(role), // send plain string to auth
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to create user in auth service: "+err.Error(),)
		return
	}

	// Step 2: Save partner_user
	partnerUser := &domain.PartnerUser{
		ID: id.GenerateID("PTNU"),
		PartnerID: req.PartnerID,
		UserID:    userResp.UserId,
		Role:      role, // store as typed enum
		Email:     req.Email,
		IsActive:  true,
	}

	if err := h.uc.CreatePartnerUser(ctx, partnerUser); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.sendNewPartnerUserNotifications(ctx, req.PartnerID, "", partnerUser, req.Password)

	response.JSON(w, http.StatusCreated, partnerUser)
}

func (h *PartnerHandler) sendNewPartnerUserNotifications(_ context.Context, partnerID string, partnerEmail string, user *domain.PartnerUser, password string) {
	if h.emailClient == nil {
		return
	}

	// Send notification to Partner
	go func(pid, recipient string, user *domain.PartnerUser) {
		// If partnerEmail is empty, fetch it
		if recipient == "" {
			partner, err := h.uc.GetPartnerByID(context.Background(), pid)
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

		_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
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
							Temporary Password: <strong>%s</strong>
						</p>
						<p style="margin-top: 30px; font-size: 14px; color: #999999;">
							Please log in and change your password immediately.<br>
							Thank you,<br>
							<strong>Pxyz Team</strong>
						</p>
					</div>
				</body>
				</html>`, pid, user.ID, user.Email, pwd)

			_, err := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
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

func (h *PartnerHandler) sendPartnerCreatedNotification(ctx context.Context, partnerUserID string) {
	if h.emailClient == nil {
		return
	}

	go func(uid string) {
		// 1. Retrieve the PartnerUser
		user, err := h.uc.GetPartnerUserByID(context.Background(), uid)
		if err != nil {
			log.Printf("[WARN] failed to fetch partner user %s for notification: %v", uid, err)
			return
		}

		// 2. Retrieve the Partner
		partner, err := h.uc.GetPartnerByID(context.Background(), user.PartnerID)
		if err != nil {
			log.Printf("[WARN] failed to fetch partner %s for notification: %v", user.PartnerID, err)
			return
		}

		if partner.ContactEmail == "" {
			log.Printf("[WARN] partner %s has no contact email; skipping notification", partner.ID)
			return
		}

		// 3. Compose and send email
		subject := "Your Partner Account Has Been Created"
		body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html><head><meta charset="UTF-8"><title>Partner Created</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">New Partner Created</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						A new partner account has been successfully created.<br>
						Partner Name: <strong>%s</strong><br>
						Partner ID: <strong>%s</strong><br>
						Partner User ID: <strong>%s</strong>
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Team</strong>
					</p>
				</div>
			</body>
			</html>`, partner.Name, partner.ID, user.ID)

		_, err = h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
			UserId:         user.ID,
			RecipientEmail: partner.ContactEmail,
			Subject:        subject,
			Body:           body,
			Type:           "partner_created",
		})
		if err != nil {
			log.Printf("[WARN] failed to send partner created notification to %s: %v", partner.ContactEmail, err)
		}
	}(partnerUserID)
}
