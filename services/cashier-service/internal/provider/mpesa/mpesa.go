// internal/provider/mpesa/mpesa.go
package mpesa

import "cashier-service/internal/domain"

type MpesaProvider struct {
	client *MpesaClient
}

func NewMpesaProvider(client *MpesaClient) *MpesaProvider {
	return &MpesaProvider{client: client}
}

func (m *MpesaProvider) Name() string {
	return "mpesa"
}

func (m *MpesaProvider) Deposit(req domain.DepositRequest) (domain.DepositResponse, error) {
	res, err := m.client.StkPush(req.Phone, req.Amount, req.AccountRef, "https://yourdomain.com/mpesa/callback")
	if err != nil {
		return domain.DepositResponse{}, err
	}
	return domain.DepositResponse{
		TransactionID: res["CheckoutRequestID"].(string),
		Status:        "pending",
	}, nil
}

func (m *MpesaProvider) Withdraw(req domain.WithdrawRequest) (domain.WithdrawResponse, error) {
	res, err := m.client.B2C(req.Phone, req.Amount, "https://yourdomain.com/mpesa/callback")
	if err != nil {
		return domain.WithdrawResponse{}, err
	}
	return domain.WithdrawResponse{
		TransactionID: res["ConversationID"].(string),
		Status:        "processing",
	}, nil
}
