package _2fahandler

import (
	"context"
	"fmt"
	"strings"

	emailclient "x/shared/email"
	"x/shared/genproto/accountpb"
	"x/shared/genproto/emailpb"

	"account-service/internal/service/2fa"
)

type TwoFAHandler struct {
	twofaService *_2faservice.TwoFAService
	emailClient *emailclient.EmailClient

}

func NewTwoFAHandler(svc *_2faservice.TwoFAService, emailClient *emailclient.EmailClient) *TwoFAHandler {
	return &TwoFAHandler{twofaService: svc, emailClient: emailClient}
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

	// Communication settings
	cChannel := req.ComChannel
	cTarget := req.ComTarget

	// Send confirmation email in background
	if cChannel == "email" && cTarget != "" && h.emailClient != nil {
		// Copy values to avoid closure issues
		userID := req.UserId
		recipient := cTarget
		codes := append([]string(nil), backupCodes...) // safe copy

		go func() {
			subject := "Two-Factor Authentication Enabled"

			// Format backup codes nicely
			var codesHTML string
			for _, code := range codes {
				codesHTML += fmt.Sprintf(`<li style="margin-bottom: 6px; font-weight: bold;">%s</li>`, code)
			}

			body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head><meta charset="UTF-8"><title>2FA Enabled</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">Two-Factor Authentication Enabled</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						You have successfully enabled <strong>Two-Factor Authentication (2FA)</strong> on your account.
					</p>
					<p style="font-size: 16px; color: #333;">
						Below are your <strong>backup recovery codes</strong>. 
						Each code can be used once if you lose access to your authenticator app:
					</p>
					<ul style="font-size: 16px; color: #2E86C1; list-style-type: none; padding: 0;">
						%s
					</ul>
					<p style="font-size: 14px; color: #b22222; font-weight: bold;">
						⚠️ Please keep these codes safe and do not share them with anyone. 
						We will never ask for them.
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Security Team</strong>
					</p>
				</div>
			</body>
			</html>
			`, codesHTML)

			_, emailErr := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         userID,
				RecipientEmail: recipient,
				Subject:        subject,
				Body:           body,
				Type:           "2fa_enabled",
			})

			if emailErr != nil {
				fmt.Printf("[WARN] failed to send 2FA enabled email to %s: %v\n", recipient, emailErr)
			}
		}()
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

	// Send email notification in background
	if req.ComChannel == "email" && req.ComTarget != "" && h.emailClient != nil {
		userID := req.UserId
		recipient := req.ComTarget

		go func() {
			subject := "Two-Factor Authentication Disabled"

			body := `
			<!DOCTYPE html>
			<html>
			<head><meta charset="UTF-8"><title>2FA Disabled</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #b22222;">Two-Factor Authentication Disabled</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						We want to inform you that <strong>Two-Factor Authentication (2FA)</strong> has been <strong>disabled</strong> on your account.
					</p>
					<p style="font-size: 14px; color: #b22222; font-weight: bold;">
						⚠️ If you did not perform this action, please secure your account immediately by resetting your password and enabling 2FA again.
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Security Team</strong>
					</p>
				</div>
			</body>
			</html>`

			_, emailErr := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         userID,
				RecipientEmail: recipient,
				Subject:        subject,
				Body:           body,
				Type:           "2fa_disabled",
			})

			if emailErr != nil {
				fmt.Printf("[WARN] failed to send 2FA disabled email to %s: %v\n", recipient, emailErr)
			}
		}()
	}

	return &accountpb.DisableTwoFAResponse{Success: ok}, nil
}


// ---------- Regenerate Backup Codes ----------
func (h *TwoFAHandler) RegenerateBackupCodes(ctx context.Context, req *accountpb.RegenerateBackupCodesRequest) (*accountpb.RegenerateBackupCodesResponse, error) {
	codes, err := h.twofaService.RegenerateBackupCodes(ctx, req.UserId, req.Method)
	if err != nil {
		return nil, err
	}

	// Prepare masked codes for gRPC response
	maskedCodes := make([]string, len(codes))
	for i, c := range codes {
		if len(c) > 3 {
			maskedCodes[i] = c[:3] + "****"
		} else {
			maskedCodes[i] = "****"
		}
	}

	// Send email notification in background with full codes
	if req.ComChannel == "email" && req.ComTarget != "" && h.emailClient != nil {
		userID := req.UserId
		recipient := req.ComTarget

		go func(fullCodes []string) {
			subject := "New 2FA Backup Codes Generated"

			// Use full codes in email body
			body := fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head><meta charset="UTF-8"><title>Backup Codes Regenerated</title></head>
			<body style="font-family: Arial, sans-serif; background-color: #f9f9f9; padding: 20px;">
				<div style="max-width: 600px; background-color: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0px 2px 5px rgba(0,0,0,0.1);">
					<h2 style="color: #2E86C1;">New 2FA Backup Codes Generated</h2>
					<p style="font-size: 16px; color: #333;">
						Hello,<br><br>
						Your <strong>Two-Factor Authentication (2FA)</strong> backup codes have been regenerated.
					</p>
					<p style="font-size: 14px; color: #555;">
						Here are your new backup codes:
					</p>
					<ul style="font-family: monospace; background: #f1f1f1; padding: 10px; border-radius: 5px; color: #2E86C1;">
						%s
					</ul>
					<p style="font-size: 14px; color: #b22222; font-weight: bold;">
						⚠️ Please store these codes safely. The old codes are no longer valid.<br>
						Do not share them with anyone.
					</p>
					<p style="margin-top: 30px; font-size: 14px; color: #999999;">
						Thank you,<br>
						<strong>Pxyz Security Team</strong>
					</p>
				</div>
			</body>
			</html>`, strings.Join(fullCodes, "<br>"))

			_, emailErr := h.emailClient.SendEmail(context.Background(), &emailpb.SendEmailRequest{
				UserId:         userID,
				RecipientEmail: recipient,
				Subject:        subject,
				Body:           body,
				Type:           "2fa_backup_regenerated",
			})

			if emailErr != nil {
				fmt.Printf("[WARN] failed to send backup codes regeneration email to %s: %v\n", recipient, emailErr)
			}
		}(codes)
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
