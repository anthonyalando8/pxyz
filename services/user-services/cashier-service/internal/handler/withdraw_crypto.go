// handler/withdrawal_crypto.go
package handler

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"cashier-service/internal/domain"
	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
	accountingpb "x/shared/genproto/shared/accounting/v1"
	"x/shared/utils/id"

	"go.uber.org/zap"
)

// buildCryptoWithdrawalContext builds context for crypto withdrawal
func (h *PaymentHandler) buildCryptoWithdrawalContext(ctx context.Context, wctx *WithdrawalContext) (*WithdrawalContext, error) {
	req := wctx.Request

	// Validate local currency is provided (must be crypto currency)
	if req.LocalCurrency == "" {
		return nil, fmt.Errorf("local_currency is required for crypto withdrawals (e.g., BTC, USDT, TRX)")
	}

	// Validate destination is crypto address
	if req.Destination == "" || len(req.Destination) < 20 {
		return nil, fmt.Errorf("valid crypto address is required as destination")
	}

	// Map currency to chain and asset
	chain, asset, err := h.mapCurrencyToCrypto(req.LocalCurrency)
	if err != nil {
		return nil, err
	}

	// Validate address format
	if err := h.validateCryptoAddress(ctx, req.Destination, chain); err != nil {
		return nil, fmt.Errorf("invalid crypto address: %v", err)
	}

	//  NEW: Check if address is internal (belongs to another user)
	internalInfo, err := h.checkIfInternalAddress(ctx, req.Destination)
	if err != nil {
		return nil, fmt.Errorf("failed to validate address: %v", err)
	}

	// Store internal address info
	wctx.InternalAddressInfo = internalInfo
	wctx.IsInternalTransfer = internalInfo != nil && internalInfo.IsInternal

	if wctx.IsInternalTransfer {
		h.logger.Info("Internal transfer detected",
			zap.String("from_user", wctx.UserID),
			zap.String("to_user", internalInfo.UserID),
			zap.String("address", req.Destination),
			zap.String("currency", req.LocalCurrency))

		//  Validate sender is not sending to their own address
		if internalInfo.UserID == wctx.UserID {
			return nil, fmt.Errorf("cannot withdraw to your own wallet address")
		}

		//  Validate chain/asset match
		if !strings.EqualFold(internalInfo.Chain, chain) || !strings.EqualFold(internalInfo.Asset, asset) {
			return nil, fmt.Errorf("address chain/asset mismatch: expected %s/%s, got %s/%s",
				chain, asset, internalInfo.Chain, internalInfo.Asset)
		}
	}

	// For crypto withdrawals: NO currency conversion
	wctx.AmountInUSD = req.Amount
	wctx.ExchangeRate = 1.0
	wctx.CryptoAddress = req.Destination
	wctx.CryptoChain = chain
	wctx.CryptoAsset = asset

	// Get user's crypto account
	userAccount, err := h.GetAccountByCurrency(ctx, wctx.UserID, "user", req.LocalCurrency, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s account: %v", req.LocalCurrency, err)
	}
	wctx.UserAccount = userAccount

	//  For internal transfers, also get recipient account
	if wctx.IsInternalTransfer {
		recipientAccount, err := h.GetAccountByCurrency(ctx, internalInfo.UserID, "user", req.LocalCurrency, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get recipient %s account: %v", req.LocalCurrency, err)
		}
		wctx.RecipientAccount = recipientAccount
	} else {
		// External transfer - get system liquidity account
		purpose := accountingpb.AccountPurpose_ACCOUNT_PURPOSE_LIQUIDITY
		systemAccount, err := h.GetAccountByCurrency(ctx, "system", "system", req.LocalCurrency, &purpose)
		if err != nil {
			return nil, fmt.Errorf("failed to get system %s liquidity account: %v", req.LocalCurrency, err)
		}
		wctx.SystemAccount = systemAccount
	}

	// Validate user balance
	balanceResp, err := h.accountingClient.Client.GetBalance(ctx, &accountingpb.GetBalanceRequest{
		Identifier: &accountingpb.GetBalanceRequest_AccountNumber{
			AccountNumber: userAccount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %v", err)
	}

	if balanceResp.Balance.AvailableBalance < req.Amount {
		return nil, fmt.Errorf("insufficient %s balance: need %.8f, have %.8f",
			req.LocalCurrency, req.Amount, balanceResp.Balance.AvailableBalance)
	}

	h.logger.Info("crypto withdrawal context built",
		zap.String("chain", chain),
		zap.String("asset", asset),
		zap.String("address", req.Destination),
		zap.String("currency", req.LocalCurrency),
		zap.Float64("amount", req.Amount),
		zap.Bool("is_internal", wctx.IsInternalTransfer),
		zap.String("user_account", userAccount))

	return wctx, nil
}

// processCryptoWithdrawal processes crypto withdrawal
// processCryptoWithdrawal processes crypto withdrawal
func (h *PaymentHandler) processCryptoWithdrawal(ctx context.Context, client *Client, wctx *WithdrawalContext) {
	req := wctx.Request

	// Create withdrawal request
	withdrawalReq := &domain.WithdrawalRequest{
		UserID:      wctx.UserIDInt,
		RequestRef:  id.GenerateTransactionID("WD-CR"),
		Amount:      req.Amount,
		Currency:    req.LocalCurrency,
		Destination: req.Destination,
		Service:     req.Service,
		Status:      domain.WithdrawalStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Store metadata
	withdrawalReq.SetOriginalAmount(req.Amount, req.LocalCurrency, 1.0)
	if withdrawalReq.Metadata == nil {
		withdrawalReq.Metadata = make(map[string]interface{})
	}
	
	//  Mark as internal or external
	if wctx.IsInternalTransfer {
		withdrawalReq.Metadata["withdrawal_type"] = "crypto_internal"
		withdrawalReq.Metadata["is_internal_transfer"] = true
		withdrawalReq.Metadata["recipient_user_id"] = wctx.InternalAddressInfo.UserID
		withdrawalReq.Metadata["recipient_wallet_id"] = wctx.InternalAddressInfo.WalletID
		withdrawalReq.Metadata["recipient_account"] = wctx.RecipientAccount
	} else {
		withdrawalReq.Metadata["withdrawal_type"] = "crypto_external"
		withdrawalReq.Metadata["is_internal_transfer"] = false
		withdrawalReq.Metadata["system_account"] = wctx.SystemAccount
	}
	
	withdrawalReq.Metadata["crypto_chain"] = wctx.CryptoChain
	withdrawalReq.Metadata["crypto_asset"] = wctx.CryptoAsset
	withdrawalReq.Metadata["crypto_address"] = wctx.CryptoAddress

	// Save to database
	if err := h.userUc.CreateWithdrawalRequest(ctx, withdrawalReq); err != nil {
		h.logger.Error("failed to create withdrawal request", zap.Error(err))
		client.SendError(fmt.Sprintf("failed to create withdrawal request: %v", err))
		return
	}

	//  Process based on transfer type
	if wctx.IsInternalTransfer {
		go h.executeInternalCryptoTransfer(withdrawalReq, wctx)
	} else {
		go h.executeCryptoWithdrawal(withdrawalReq, wctx)
	}

	// Send success response
	client.SendSuccess("crypto withdrawal request created", map[string]interface{}{
		"request_ref":         withdrawalReq.RequestRef,
		"amount":              req.Amount,
		"currency":            req.LocalCurrency,
		"chain":               wctx.CryptoChain,
		"asset":               wctx.CryptoAsset,
		"address":             wctx.CryptoAddress,
		"withdrawal_type":     withdrawalReq.Metadata["withdrawal_type"],
		"is_internal":         wctx.IsInternalTransfer,
		"status":              "processing",
	})
}

// executeInternalCryptoTransfer handles internal crypto transfers (no blockchain interaction)
func (h *PaymentHandler) executeInternalCryptoTransfer(withdrawal *domain.WithdrawalRequest, wctx *WithdrawalContext) {
	ctx := context.Background()

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[InternalCryptoTransfer] Failed to mark as processing: %v", err)
		return
	}

	h.logger.Info("Executing internal crypto transfer",
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("from_user", wctx.UserID),
		zap.String("to_user", wctx.InternalAddressInfo.UserID),
		zap.String("currency", withdrawal.Currency),
		zap.Float64("amount", withdrawal.Amount))

	//  Direct transfer from sender to recipient (no blockchain, no system account)
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   wctx.UserAccount,
		ToAccountNumber:     wctx.RecipientAccount, //  Direct to recipient
		Amount:              withdrawal.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Internal crypto transfer %.8f %s to user %s",
			withdrawal.Amount, withdrawal.Currency, wctx.InternalAddressInfo.UserID),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
		ToAddress:           &wctx.CryptoAddress,
	}

	transferResp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		errMsg := fmt.Sprintf("internal transfer failed: %v", err)
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		// Notify sender
		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
			"type": "withdrawal_failed",
			"data": {
				"request_ref": "%s",
				"error": "%s"
			}
		}`, withdrawal.RequestRef, errMsg)))
		return
	}

	h.logger.Info("Internal crypto transfer completed",
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("receipt_code", transferResp.ReceiptCode),
		zap.Int64("journal_id", transferResp.JournalId))

	//  Update withdrawal as completed (no blockchain confirmation needed)
	if withdrawal.Metadata == nil {
		withdrawal.Metadata = make(map[string]interface{})
	}
	withdrawal.Metadata["receipt_code"] = transferResp.ReceiptCode
	withdrawal.Metadata["journal_id"] = transferResp.JournalId
	withdrawal.Metadata["transfer_type"] = "internal"
	withdrawal.Metadata["completed_at"] = time.Now()

	h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "completed", nil)

	//  Notify sender
	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
		"type": "withdrawal_completed",
		"data": {
			"request_ref": "%s",
			"receipt_code": "%s",
			"amount": %.8f,
			"currency": "%s",
			"to_address": "%s",
			"recipient_user_id": "%s",
			"transfer_type": "internal",
			"status": "completed",
			"message": "Internal transfer completed instantly"
		}
	}`, withdrawal.RequestRef, transferResp.ReceiptCode,
		withdrawal.Amount, withdrawal.Currency,
		wctx.CryptoAddress, wctx.InternalAddressInfo.UserID)))

	//  Notify recipient
	h.hub.SendToUser(wctx.InternalAddressInfo.UserID, []byte(fmt.Sprintf(`{
		"type": "crypto_received",
		"data": {
			"receipt_code": "%s",
			"amount": %.8f,
			"currency": "%s",
			"from_user_id": "%s",
			"to_address": "%s",
			"wallet_id": %d,
			"transfer_type": "internal",
			"message": "You received %.8f %s from user %s"
		}
	}`, transferResp.ReceiptCode,
		withdrawal.Amount, withdrawal.Currency,
		wctx.UserID, wctx.CryptoAddress,
		wctx.InternalAddressInfo.WalletID,
		withdrawal.Amount, withdrawal.Currency, wctx.UserID)))
}

// executeCryptoWithdrawal executes the crypto withdrawal
// handler/withdrawal_crypto.go

func (h *PaymentHandler) executeCryptoWithdrawal(withdrawal *domain.WithdrawalRequest, wctx *WithdrawalContext) {
	ctx := context.Background()

	if wctx.IsInternalTransfer {
		h.logger.Error("executeCryptoWithdrawal called for internal transfer",
			zap.String("request_ref", withdrawal.RequestRef))
		return
	}

	if err := h.userUc.MarkWithdrawalProcessing(ctx, withdrawal.RequestRef); err != nil {
		log.Printf("[CryptoWithdrawal] Failed to mark as processing: %v", err)
		return
	}

	h.logger.Info("Executing crypto withdrawal",
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("chain", wctx.CryptoChain),
		zap.String("asset", wctx.CryptoAsset),
		zap.String("to_address", wctx.CryptoAddress),
		zap.Float64("amount", withdrawal.Amount))

	//  Step 1: Transfer from user account to system liquidity account
	transferReq := &accountingpb.TransferRequest{
		FromAccountNumber:   wctx.UserAccount,
		ToAccountNumber:     wctx.SystemAccount,
		Amount:              withdrawal.Amount,
		AccountType:         accountingpb.AccountType_ACCOUNT_TYPE_REAL,
		Description:         fmt.Sprintf("Crypto withdrawal %.8f %s to %s",
			withdrawal.Amount, withdrawal.Currency, wctx.CryptoAddress),
		ExternalRef:         &withdrawal.RequestRef,
		CreatedByExternalId: fmt.Sprintf("%d", withdrawal.UserID),
		CreatedByType:       accountingpb.OwnerType_OWNER_TYPE_USER,
		TransactionType:     accountingpb.TransactionType_TRANSACTION_TYPE_WITHDRAWAL,
		ToAddress:           &wctx.CryptoAddress, //  Pass destination address
	}

	transferResp, err := h.accountingClient.Client.Transfer(ctx, transferReq)
	if err != nil {
		errMsg := fmt.Sprintf("transfer failed: %v", err)
		h.userUc.FailWithdrawal(ctx, withdrawal.RequestRef, errMsg)

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_failed",
            "data": {
                "request_ref": "%s",
                "error": "%s"
            }
        }`, withdrawal.RequestRef, errMsg)))
		return
	}

	h.logger.Info("Transfer to system account completed",
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("receipt_code", transferResp.ReceiptCode),
		zap.Int64("journal_id", transferResp.JournalId))

	payableAmount := transferResp.PayableAmount
	if payableAmount != 0 {
		h.logger.Info("Transfer payable amount",
			zap.Float64("payable_amount", payableAmount))
	}else{
		h.logger.Info("No payable amount in transfer response")
		payableAmount = withdrawal.Amount
	}

	//  Step 2: Convert amount to smallest unit for blockchain
	amountInSmallestUnit := h.convertToSmallestUnit(payableAmount, withdrawal.Currency)
	accountingTxID := transferResp.ReceiptCode
	if accountingTxID == "" {
		accountingTxID = withdrawal.RequestRef
	}

	//  Step 3: Call crypto service to execute withdrawal
	cryptoResp, err := h.cryptoClient.TransactionClient.Withdraw(ctx, &cryptopb.WithdrawRequest{
		AccountingTxId: accountingTxID,              //  Idempotency key from accounting
		UserId:         fmt.Sprintf("%d", withdrawal.UserID),
		Chain:          mapChainToProto(wctx.CryptoChain),
		Asset:          wctx.CryptoAsset,
		Amount:         amountInSmallestUnit,              //  In smallest unit (satoshis, SUN, etc.)
		ToAddress:      wctx.CryptoAddress,
		Memo:           "",                                 // Optional memo (for chains that support it)
	})

	if err != nil {
		//errMsg := fmt.Sprintf("blockchain withdrawal failed: %v", err)
		h.logger.Error("Blockchain withdrawal failed",
			zap.String("request_ref", withdrawal.RequestRef),
			zap.Error(err))

		// ⚠️ Money already transferred to system account
		// Mark for manual review
		h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "blockchain_failed", nil)

		if withdrawal.Metadata == nil {
			withdrawal.Metadata = make(map[string]interface{})
		}
		withdrawal.Metadata["receipt_code"] = transferResp.ReceiptCode
		withdrawal.Metadata["journal_id"] = transferResp.JournalId
		withdrawal.Metadata["blockchain_error"] = err.Error()
		withdrawal.Metadata["requires_manual_review"] = true

		h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
            "type": "withdrawal_processing",
            "data": {
                "request_ref": "%s",
                "receipt_code": "%s",
                "message": "Withdrawal is being processed. Blockchain transaction pending.",
                "status": "blockchain_pending"
            }
        }`, withdrawal.RequestRef, transferResp.ReceiptCode)))
		return
	}

	//  Extract transaction details from response
	tx := cryptoResp.Transaction
	if tx == nil {
		h.logger.Error("No transaction returned from crypto service",
			zap.String("request_ref", withdrawal.RequestRef))
		h.userUc.UpdateWithdrawalStatus(ctx, withdrawal.RequestRef, "blockchain_failed", nil)
		return
	}

	h.logger.Info("Blockchain withdrawal initiated",
		zap.String("request_ref", withdrawal.RequestRef),
		zap.String("transaction_id", tx.TransactionId),
		zap.String("tx_hash", tx.TxHash),
		zap.String("status", tx.Status.String()))

	//  Step 4: Update withdrawal with transaction details
	// h.userUc.UpdateWithdrawalWithCryptoTx(ctx, withdrawal.ID,
	// 	transferResp.ReceiptCode,
	// 	transferResp.JournalId,
	// 	tx.TxHash,
	// 	tx.Status.String())

	// Store all metadata
	if withdrawal.Metadata == nil {
		withdrawal.Metadata = make(map[string]interface{})
	}
	withdrawal.Metadata["receipt_code"] = transferResp.ReceiptCode
	withdrawal.Metadata["journal_id"] = transferResp.JournalId
	withdrawal.Metadata["crypto_transaction_id"] = tx.TransactionId
	withdrawal.Metadata["tx_hash"] = tx.TxHash
	withdrawal.Metadata["blockchain_status"] = tx.Status.String()
	withdrawal.Metadata["network_fee"] = tx.NetworkFee.Amount
	withdrawal.Metadata["network_fee_currency"] = tx.NetworkFee.Currency
	withdrawal.Metadata["confirmations"] = tx.Confirmations
	withdrawal.Metadata["required_confirmations"] = tx.RequiredConfirmations
	withdrawal.Metadata["blockchain_initiated_at"] = time.Now()

	//  Step 5: Monitor transaction status (in background)
	go h.monitorCryptoTransaction(withdrawal.RequestRef, tx.TransactionId, wctx.CryptoChain)

	// Notify user
	h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
        "type": "withdrawal_blockchain_initiated",
        "data": {
            "request_ref": "%s",
            "receipt_code": "%s",
            "transaction_id": "%s",
            "tx_hash": "%s",
            "amount": %.8f,
            "currency": "%s",
            "network_fee": "%s",
            "network_fee_currency": "%s",
            "to_address": "%s",
            "chain": "%s",
            "status": "%s",
            "confirmations": %d,
            "required_confirmations": %d,
            "explorer_url": "%s",
            "message": "%s"
        }
    }`, withdrawal.RequestRef, transferResp.ReceiptCode, tx.TransactionId, tx.TxHash,
		withdrawal.Amount, withdrawal.Currency,
		tx.NetworkFee.Amount, tx.NetworkFee.Currency,
		wctx.CryptoAddress, wctx.CryptoChain,
		tx.Status.String(), tx.Confirmations, tx.RequiredConfirmations,
		h.getBlockExplorerURL(wctx.CryptoChain, tx.TxHash),
		cryptoResp.Message)))
}

//  Updated monitorCryptoTransaction - uses transaction_id instead of tx_hash
func (h *PaymentHandler) monitorCryptoTransaction(requestRef, transactionID, chain string) {
	ctx := context.Background()
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	timeout := time.After(30 * time.Minute) // Timeout after 30 minutes

	for {
		select {
		case <-timeout:
			h.logger.Warn("Transaction monitoring timeout",
				zap.String("request_ref", requestRef),
				zap.String("transaction_id", transactionID))

			h.userUc.UpdateWithdrawalStatus(ctx, requestRef, "blockchain_timeout", nil)
			return

		case <-ticker.C:
			//  Check transaction status using transaction_id
			statusResp, err := h.cryptoClient.TransactionClient.GetTransactionStatus(ctx, &cryptopb.GetTransactionStatusRequest{
				TransactionId: transactionID, //  Use transaction_id from crypto service
			})

			if err != nil {
				h.logger.Warn("Failed to get transaction status",
					zap.String("transaction_id", transactionID),
					zap.Error(err))
				continue
			}

			h.logger.Info("Transaction status checked",
				zap.String("transaction_id", transactionID),
				zap.String("tx_hash", statusResp.TxHash),
				zap.String("status", statusResp.Status.String()),
				zap.Int32("confirmations", statusResp.Confirmations),
				zap.Int32("required_confirmations", statusResp.RequiredConfirmations))

			// Update withdrawal status based on blockchain status
			switch statusResp.Status {
			case cryptopb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED,
				cryptopb.TransactionStatus_TRANSACTION_STATUS_COMPLETED:
				// Transaction confirmed!
				h.userUc.UpdateWithdrawalStatus(ctx, requestRef, "completed", nil)

				// Get withdrawal to notify user
				withdrawal, err := h.userUc.GetWithdrawalByRef(ctx, requestRef)
				if err == nil {
					h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
                        "type": "withdrawal_completed",
                        "data": {
                            "request_ref": "%s",
                            "transaction_id": "%s",
                            "tx_hash": "%s",
                            "confirmations": %d,
                            "required_confirmations": %d,
                            "status": "completed",
                            "status_message": "%s"
                        }
                    }`, requestRef, transactionID, statusResp.TxHash,
						statusResp.Confirmations, statusResp.RequiredConfirmations,
						statusResp.StatusMessage)))
				}

				return // Stop monitoring

			case cryptopb.TransactionStatus_TRANSACTION_STATUS_FAILED,
				cryptopb.TransactionStatus_TRANSACTION_STATUS_CANCELLED:
				// Transaction failed!
				h.userUc.UpdateWithdrawalStatus(ctx, requestRef, "blockchain_failed", nil)

				withdrawal, err := h.userUc.GetWithdrawalByRef(ctx, requestRef)
				if err == nil {
					h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
                        "type": "withdrawal_failed",
                        "data": {
                            "request_ref": "%s",
                            "transaction_id": "%s",
                            "tx_hash": "%s",
                            "error": "Blockchain transaction failed: %s",
                            "status": "failed"
                        }
                    }`, requestRef, transactionID, statusResp.TxHash, statusResp.StatusMessage)))
				}

				return // Stop monitoring

			case cryptopb.TransactionStatus_TRANSACTION_STATUS_PENDING,
				cryptopb.TransactionStatus_TRANSACTION_STATUS_BROADCASTED,
				cryptopb.TransactionStatus_TRANSACTION_STATUS_CONFIRMING:
				// Still pending, continue monitoring
				
				// Optionally notify user of progress
				if statusResp.Confirmations > 0 {
					withdrawal, err := h.userUc.GetWithdrawalByRef(ctx, requestRef)
					if err == nil {
						h.hub.SendToUser(fmt.Sprintf("%d", withdrawal.UserID), []byte(fmt.Sprintf(`{
                            "type": "withdrawal_confirming",
                            "data": {
                                "request_ref": "%s",
                                "transaction_id": "%s",
                                "tx_hash": "%s",
                                "confirmations": %d,
                                "required_confirmations": %d,
                                "status": "confirming"
                            }
                        }`, requestRef, transactionID, statusResp.TxHash,
							statusResp.Confirmations, statusResp.RequiredConfirmations)))
					}
				}
				
				continue // Keep monitoring
			}
		}
	}
}

//  Helper: Convert amount to smallest unit
func (h *PaymentHandler) convertToSmallestUnit(amount float64, currency string) string {
	decimals := map[string]int{
		"BTC":  8,  // satoshis
		"TRX":  6,  // SUN
		"USDT": 6,  // SUN (on TRON)
		"ETH":  18, // wei
		"USDC": 6,  // (on Ethereum)
	}

	dec := 8 // default
	if d, ok := decimals[strings.ToUpper(currency)]; ok {
		dec = d
	}

	// Convert to smallest unit
	multiplier := math.Pow(10, float64(dec))
	smallestUnit := int64(amount * multiplier)

	return fmt.Sprintf("%d", smallestUnit)
}

// getBlockExplorerURL returns the block explorer URL for a transaction
func (h *PaymentHandler) getBlockExplorerURL(chain, txHash string) string {
	if txHash == "" {
		return ""
	}

	explorers := map[string]string{
		"BITCOIN":  "https://blockstream.info/tx/",
		"TRON":     "https://tronscan.org/#/transaction/",
		"ETHEREUM": "https://etherscan.io/tx/",
	}

	if baseURL, ok := explorers[strings.ToUpper(chain)]; ok {
		return baseURL + txHash
	}

	return ""
}


// handler/withdrawal_crypto.go

// checkIfInternalAddress checks if withdrawal address belongs to a user in the system
func (h *PaymentHandler) checkIfInternalAddress(ctx context.Context, address string) (*InternalAddressInfo, error) {
	// Call crypto service to check if address exists in our system
	walletResp, err := h.cryptoClient.WalletClient.GetWalletByAddress(ctx, &cryptopb.GetWalletByAddressRequest{
		Address: address,
	})

	if err != nil {
		// Address not found in system = external address
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no wallet") {
			return nil, nil // Not an internal address
		}
		// Other error
		return nil, fmt.Errorf("failed to check address: %w", err)
	}

	// Address found - it's an internal wallet
	wallet := walletResp.Wallet
	if wallet == nil {
		return nil, nil
	}

	h.logger.Info("Internal address detected",
		zap.String("address", address),
		zap.String("owner_user_id", wallet.UserId),
		zap.String("chain", wallet.Chain.String()),
		zap.String("asset", wallet.Asset))

	chain := mapProtoToChain(wallet.Chain)
	return &InternalAddressInfo{
		IsInternal:   true,
		UserID:       wallet.UserId,
		WalletID:     wallet.Id,
		Chain:        	chain,
		Asset:        wallet.Asset,
		WalletObject: wallet,
	}, nil
}

// InternalAddressInfo holds information about an internal wallet address
type InternalAddressInfo struct {
	IsInternal   bool
	UserID       string
	WalletID     int64
	Chain        string
	Asset        string
	WalletObject *cryptopb.Wallet
}