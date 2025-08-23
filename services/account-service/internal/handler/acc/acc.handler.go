package _2fahandler

import (
	//"context"
	emailclient "x/shared/email"
	//"x/shared/genproto/accountpb"

	"account-service/internal/service/acc"
)

type AccountHandler struct {
	accService *service.AccountService
	emailClient *emailclient.EmailClient
}

func NewAccountHandler(svc *service.AccountService, emailClient *emailclient.EmailClient) *AccountHandler {
	return &AccountHandler{accService: svc, emailClient: emailClient}
}


// func (h *AccountHandler) GetAccountHandler(ctx context.Context, req *accountpb.) (*accountpb.InitiateTOTPSetupResponse, error) {
// 	secret, otpURL, err := h.accService.GetOrCreateProfile(ctx, req.UserID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &accountpb.InitiateTOTPSetupResponse{
// 		Secret: secret,
// 		OtpUrl: otpURL,
// 	}, nil
// }