package _2fahandler

import (
	"context"
	"log"

	emailclient "x/shared/email"
	"x/shared/genproto/accountpb"

	notificationclient "x/shared/notification"
	notificationpb "x/shared/genproto/shared/notificationpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	"account-service/internal/service/2fa"
	"x/shared/utils/notification"
)

type TwoFAHandler struct {
	twofaService *_2faservice.TwoFAService
	emailClient *emailclient.EmailClient
	notificationClient *notificationclient.NotificationService
}

func NewTwoFAHandler(svc *_2faservice.TwoFAService, emailClient *emailclient.EmailClient, 	notificationClient *notificationclient.NotificationService) *TwoFAHandler {
	return &TwoFAHandler{twofaService: svc, emailClient: emailClient, notificationClient: notificationClient,}
}

// ---------- Initiate TOTP Setup ----------
func (h *TwoFAHandler) InitiateTOTPSetup(ctx context.Context, req *accountpb.InitiateTOTPSetupRequest) (*accountpb.InitiateTOTPSetupResponse, error) {
	secret, otpURL, err := h.twofaService.InitiateTOTPSetup(ctx, req.UserId, req.Email)
	if err != nil {
		return nil, err
	}

	return &accountpb.InitiateTOTPSetupResponse{
		Secret: secret,
		OtpUrl: otpURL,
	}, nil
}

// ---------- Enable 2FA ----------
func (h *TwoFAHandler) EnableTwoFA(ctx context.Context, req *accountpb.EnableTwoFARequest) (*accountpb.EnableTwoFAResponse, error) {
	_, backupCodes, err := h.twofaService.EnableTwoFA(ctx, req.UserId, req.Method, req.Secret, req.Code)
	if err != nil {
		return nil, err
	}

	// Send confirmation notification in background if email is used
	if req.ComChannel == "email" && req.ComTarget != "" && h.notificationClient != nil {
		payload := map[string]interface{}{
			"RecoveryCodes": backupCodes,
		}
		h.sendNotification(req.UserId, "2FA_ENABLED", req.ComTarget, "", payload, []string{"email", "ws"})
	}

	// Mask backup codes in API response
	masked := make([]string, len(backupCodes))
	for i := range masked {
		masked[i] = "********"
	}

	return &accountpb.EnableTwoFAResponse{
		Success:     true,
		BackupCodes: masked,
	}, nil
}


// Helper to send notification
func (h *TwoFAHandler) sendNotification(userID, eventType, recipientEmail, recipientPhone string, payload map[string]interface{}, channels []string) {
	go func() {
		ctx := context.Background() // background context for async sending

		notif := []*notificationpb.Notification{
			{
				RequestId:   uuid.New().String(),
				OwnerType:   "user",
				OwnerId:     userID,
				EventType:   eventType,
				Title:       "2FA Services",
				Body:        "Dear user you have a 2FA status update",
				ChannelHint: channels,
				Payload: func() *structpb.Struct {
					fixed := utils.NormalizePayload(payload)
					s, err := structpb.NewStruct(fixed)
					if err != nil {
						log.Printf("[2FA][Payload Conversion Failed] userID=%s err=%v payload=%+v", userID, err, fixed)
						return nil
					}
					log.Printf("[2FA][Payload Submitted] userID=%s payload=%+v", userID, fixed)
					return s
				}(),
				VisibleInApp: true,
				Priority:     "high",
				Status:       "pending",
			},
		}

		// Set recipient fields on the first element of the slice
		if len(notif) > 0 {
			if recipientEmail != "" {
				notif[0].RecipientEmail = recipientEmail
			}
			if recipientPhone != "" {
				notif[0].RecipientPhone = recipientPhone
			}
		}


		_, err := h.notificationClient.Client.CreateNotification(ctx, &notificationpb.CreateNotificationsRequest{
			Notifications: notif,
		})
		if err != nil {
			log.Printf("[WARN] failed to send %s notification to user=%s: %v", eventType, userID, err)
		} else {
			log.Printf("Successfully queued %s notification | User=%s", eventType, userID)
		}
	}()
}

// ---------- Get 2FA Status ----------
func (h *TwoFAHandler) GetTwoFAStatus(ctx context.Context, req *accountpb.GetTwoFAStatusRequest) (*accountpb.GetTwoFAStatusResponse, error) {
	status, method, err := h.twofaService.GetTwoFAStatus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return &accountpb.GetTwoFAStatusResponse{
		IsEnabled: status,
		Method:    method,
	}, nil
}

// ---------- Verify 2FA ----------
func (h *TwoFAHandler) VerifyTwoFA(ctx context.Context, req *accountpb.VerifyTwoFARequest) (*accountpb.VerifyTwoFAResponse, error) {
	ok, err := h.twofaService.VerifyTwoFA(ctx, req.UserId, req.Method, req.Code, req.BackupCode)
	if err != nil {
		return nil, err
	}

	return &accountpb.VerifyTwoFAResponse{Success: ok}, nil
}

// ---------- Disable 2FA ----------
func (h *TwoFAHandler) DisableTwoFA(ctx context.Context, req *accountpb.DisableTwoFARequest) (*accountpb.DisableTwoFAResponse, error) {
	ok, err := h.twofaService.DisableTwoFA(ctx, req.UserId, req.Method, req.Code, req.BackupCode)
	if err != nil {
		return nil, err
	}

	// Send email notification in background using helper
	if req.ComChannel == "email" && req.ComTarget != "" && h.notificationClient != nil {
		payload := map[string]interface{}{} // no dynamic fields needed for this template
		h.sendNotification(req.UserId, "2FA_DISABLED", req.ComTarget, "", payload, []string{"email", "ws"})
	}

	return &accountpb.DisableTwoFAResponse{Success: ok}, nil
}


// ---------- Regenerate Backup Codes ----------
func (h *TwoFAHandler) RegenerateBackupCodes(ctx context.Context, req *accountpb.RegenerateBackupCodesRequest) (*accountpb.RegenerateBackupCodesResponse, error) {
	codes, err := h.twofaService.RegenerateBackupCodes(ctx, req.UserId, req.Method)
	if err != nil {
		return nil, err
	}

	// Mask backup codes for gRPC response
	maskedCodes := make([]string, len(codes))
	for i, c := range codes {
		if len(c) > 3 {
			maskedCodes[i] = c[:3] + "****"
		} else {
			maskedCodes[i] = "****"
		}
	}

	// Send notification in background using the helper
	if req.ComChannel == "email" && req.ComTarget != "" && h.notificationClient != nil {
		payload := map[string]interface{}{
			"BackupCodes": codes, // full codes for template rendering
		}
		h.sendNotification(req.UserId, "2FA_BACKUP_CODE_REGN", req.ComTarget, "", payload, []string{"email", "ws"})
	}

	return &accountpb.RegenerateBackupCodesResponse{
		Success:     true,
		BackupCodes: maskedCodes, // only masked codes returned via RPC
	}, nil
}

// ---------- Get Backup Codes ----------
func (h *TwoFAHandler) GetBackupCodes(ctx context.Context, req *accountpb.GetBackupCodesRequest) (*accountpb.GetBackupCodesResponse, error) {
	codes, err := h.twofaService.GetBackupCodes(ctx, req.UserId, req.Method)
	if err != nil {
		return nil, err
	}

	return &accountpb.GetBackupCodesResponse{BackupCodes: codes}, nil
}
