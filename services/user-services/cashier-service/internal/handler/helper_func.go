package handler

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func ptrToStr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// func (h *PaymentHandler) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
// 	var req struct {
// 		Amount   float64 `json:"amount"`
// 		Method   string  `json:"method"`
// 		Service  string  `json:"service"`  // service to pick partner from
// 		Currency string  `json:"currency"` // user wallet currency
// 	}
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		response.Error(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
// 		return
// 	}

// 	// For now, assume user withdraws from USD wallet → partner KES account
// 	req.Currency = "USD"     // user wallet currency
// 	withdrawCurrency := "KES" // partner account currency

// 	// Get authenticated user
// 	userID, ok := r.Context().Value(middleware.ContextUserID).(string)
// 	if !ok || userID == "" {
// 		response.Error(w, http.StatusUnauthorized, "user not authenticated")
// 		return
// 	}

// 	// Get both accounts concurrently
// 	partnerAccount, userAccount, err := h.GetPartnerAndUserAccounts(r.Context(), req.Service, req.Currency, userID)
// 	if err != nil {
// 		response.Error(w, http.StatusBadRequest, "your request has been declined: "+err.Error())
// 		return
// 	}

// 	// Parse userID to int64 for gRPC
// 	userIDInt, err := strconv.ParseInt(userID, 10, 64)
// 	if err != nil {
// 		response.Error(w, http.StatusBadRequest, "invalid user ID")
// 		return
// 	}

// 	// Construct accounting transaction (User → Partner)
// 	grpcReq := &accountingpb.CreateTransactionRequest{
// 		Description:   fmt.Sprintf("Withdrawal by user %s via %s", userID, req.Method),
// 		CreatedByType: accountingpb.OwnerType_USER,
// 		CreatedByUser: userIDInt,
// 		DepositCurrency: withdrawCurrency,
// 		Entries: []*accountingpb.TransactionEntry{
// 			{
// 				AccountNumber: userAccount,
// 				DrCr:          accountingpb.DrCr_DR, // Debit user wallet (money leaves)
// 				Amount:        req.Amount,
// 				Currency:      req.Currency,
// 			},
// 			{
// 				AccountNumber: partnerAccount,
// 				DrCr:          accountingpb.DrCr_CR, // Credit partner account (money arrives in M-Pesa)
// 				Amount:        req.Amount,
// 				Currency:      req.Currency,
// 			},
// 		},
// 	}

// 	// Call gRPC to post transaction
// 	resp, err := h.accountingClient.Client.PostTransaction(r.Context(), grpcReq)
// 	if err != nil {
// 		response.Error(w, http.StatusBadGateway, "failed to post transaction: "+err.Error())
// 		return
// 	}

// 	// Success response
// 	response.JSON(w, http.StatusOK, map[string]interface{}{
// 		"message":         "withdrawal successful",
// 		"user_account":    userAccount,
// 		"partner_account": partnerAccount,
// 		"transaction":     resp,
// 	})
// }
