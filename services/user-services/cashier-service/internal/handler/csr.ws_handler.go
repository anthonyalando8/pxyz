package handler

import (
	"context"
	"fmt"
)

func (h *PaymentHandler) HandleWSMessage(client *Client, msg *WSMessage) {
	ctx := context.Background()

	switch msg.Type {
		// ========== Verification Operations ==========
	case "verification_request":
		h.handleVerificationRequest(ctx, client, msg. Data)

	case "verify_totp":
		h. handleVerifyTOTP(ctx, client, msg.Data)

	case "verify_otp":
		h.handleVerifyOTP(ctx, client, msg. Data)
	// ========== Partner Operations ==========
	case "get_partners":
		h.handleGetPartners(ctx, client, msg.Data)

	// ========== Account Operations ==========
	case "get_accounts":
		h.handleGetAccounts(ctx, client)

	case "get_account_balance":
		h.handleGetAccountBalance(ctx, client, msg.Data)

	case "get_owner_summary":
		h.handleGetOwnerSummary(ctx, client)

	case "create_account":  // ✅ NEW
		h.handleCreateAccount(ctx, client, msg.Data)

	case "get_supported_currencies":  // ✅ NEW
		h.handleGetSupportedCurrencies(ctx, client)

	// ========== Deposit/Withdrawal Operations ==========
	case "deposit_request":
		h.handleDepositRequest(ctx, client, msg.Data)

	case "withdraw_request":
		h.handleWithdrawRequest(ctx, client, msg.Data)

	case "get_deposit_status":
		h.handleGetDepositStatus(ctx, client, msg.Data)

	case "cancel_deposit":
		h.handleCancelDeposit(ctx, client, msg.Data)

	// ========== Transaction History ==========
	case "get_history":
		h.handleGetHistory(ctx, client, msg.Data)

	case "get_transaction_by_receipt":
		h.handleGetTransactionByReceipt(ctx, client, msg.Data)

	// ========== Statements & Reports ==========
	case "get_account_statement":
		h.handleGetAccountStatement(ctx, client, msg.Data)

	case "get_owner_statement":
		h.handleGetOwnerStatement(ctx, client, msg.Data)

	case "get_ledgers":
		h.handleGetLedgers(ctx, client, msg.Data)

	// ========== P2P Transfer ==========
	case "transfer":
		h.handleTransfer(ctx, client, msg.Data)

	// ========== Fee Calculation ==========
	case "calculate_fee":
		h.handleCalculateFee(ctx, client, msg.Data)

	case "convert_and_transfer":
		h.handleConvertAndTransfer(ctx, client, msg.Data)

	default:
		client.SendError(fmt.Sprintf("unknown message type: %s", msg.Type))
	}
}

