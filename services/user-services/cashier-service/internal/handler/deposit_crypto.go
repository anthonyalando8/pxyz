// handler/deposit_crypto.go
package handler

import (
	"context"
	"fmt"
	"strings"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
)

// buildCryptoDepositContext builds context for crypto deposit
func (h *PaymentHandler) buildCryptoDepositContext(ctx context.Context, dctx *DepositContext) (*DepositContext, error) {
	req := dctx.Request

	//  Validate local currency is provided (must be crypto currency)
	if req.LocalCurrency == "" {
		return nil, fmt.Errorf("local_currency is required for crypto deposits (e.g., BTC, USDT, TRX)")
	}

	//  Map currency to chain and asset
	chain, asset, err := h.mapCurrencyToCrypto(req.LocalCurrency)
	if err != nil {
		return nil, err
	}

	dctx.CryptoChain = chain
	dctx.CryptoAsset = asset

	//  Get or create user's wallet for this chain/asset
	chainProto := mapChainToProto(chain)
	walletResp, err := h.cryptoClient.WalletClient.GetUserWallets(ctx, &cryptopb.GetUserWalletsRequest{
		UserId:          dctx.UserID,
		Chain:           chainProto,
		Asset:           asset,
		ActiveOnly:      true,
		CreateIfMissing: true, //  Auto-create if doesn't exist
	})

	if err != nil {
		h.logger.Error("failed to get crypto wallet",
			zap.String("user_id", dctx.UserID),
			zap.String("chain", chain),
			zap.String("asset", asset),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get crypto wallet: %v", err)
	}

	if len(walletResp.Wallets) == 0 {
		return nil, fmt.Errorf("no wallet available for %s on %s", asset, chain)
	}

	// Use first wallet
	wallet := walletResp.Wallets[0]
	dctx.CryptoWallet = wallet

	//  For crypto deposits: Amount is in crypto units (no conversion to USD)
	dctx.AmountInUSD = req.Amount // Store as-is in crypto units
	dctx.ExchangeRate = 1.0       // No conversion
	dctx.LocalCurrency = req.LocalCurrency
	dctx.TargetCurrency = req.LocalCurrency // Same as local

	h.logger.Info("crypto deposit context built",
		zap.String("chain", chain),
		zap.String("asset", asset),
		zap.Int64("wallet_id", wallet.Id),
		zap.String("address", wallet.Address),
		zap.Float64("amount", req.Amount))

	return dctx, nil
}

// processCryptoDeposit processes crypto deposit
//  NO DATABASE RECORD - Just return wallet address
func (h *PaymentHandler) processCryptoDeposit(ctx context.Context, client *Client, dctx *DepositContext) {
	req := dctx.Request
	wallet := dctx.CryptoWallet

	h.logger.Info("crypto deposit - returning wallet address",
		zap.String("user_id", dctx.UserID),
		zap.String("chain", dctx.CryptoChain),
		zap.String("asset", dctx.CryptoAsset),
		zap.String("address", wallet.Address),
		zap.Float64("amount_requested", req.Amount)) // May be 0

	//  Build response with deposit instructions
	response := map[string]interface{}{
		"deposit_type":         "crypto",
		"chain":                dctx.CryptoChain,
		"asset":                dctx.CryptoAsset,
		"currency":             req.LocalCurrency,
		"wallet_id":            wallet.Id,
		"address":              wallet.Address,
		"qr_code_url":          h.generateQRCodeURL(wallet.Address, req.Amount, dctx.CryptoAsset),
		"instructions":         h.getCryptoDepositInstructions(dctx.CryptoChain, dctx.CryptoAsset),
		"explorer_url":         h.getBlockExplorerAddressURL(dctx.CryptoChain, wallet.Address),
		"minimum_deposit":      h.getMinimumDeposit(dctx.CryptoChain, dctx.CryptoAsset),
		"required_confirmations": h.getRequiredConfirmations(dctx.CryptoChain),
	}

	//  Only include amount if user provided it
	if req.Amount > 0 {
		response["amount_requested"] = req.Amount

		//  Get network fee estimate only if amount provided
		feeResp, err := h.cryptoClient.TransactionClient.EstimateNetworkFee(ctx, &cryptopb.EstimateNetworkFeeRequest{
			Chain:     mapChainToProto(dctx.CryptoChain),
			Asset:     dctx.CryptoAsset,
			Amount:    fmt.Sprintf("%.8f", req.Amount),
			ToAddress: wallet.Address,
		})

		if err == nil {
			response["network_fee_info"] = map[string]interface{}{
				"fee_amount":        feeResp.FeeAmount,
				"fee_currency":      feeResp.FeeCurrency,
				"fee_formatted":     feeResp.FeeFormatted,
				"estimated_at":      feeResp.EstimatedAt.AsTime(),
				"valid_for_seconds": feeResp.ValidFor,
				"explanation":       feeResp.Explanation,
			}
		} else {
			h.logger.Warn("failed to estimate network fee",
				zap.Error(err))
		}
	} else {
		//  If no amount provided, add note
		response["note"] = "Deposit any amount above the minimum. Network fees apply."
	}

	// Add chain-specific info
	if memo := h.getChainMemo(dctx.CryptoChain, wallet); memo != "" {
		response["memo"] = memo
		response["memo_required"] = true
	}

	client.SendSuccess("crypto deposit address retrieved", response)

	h.logger.Info("crypto deposit address returned to user",
		zap.String("user_id", dctx.UserID),
		zap.String("address", wallet.Address),
		zap.String("chain", dctx.CryptoChain),
		zap.String("asset", dctx.CryptoAsset),
		zap.Bool("amount_provided", req.Amount > 0))
}

// ============================================================================
// CRYPTO HELPER FUNCTIONS
// ============================================================================

// getCryptoDepositInstructions returns deposit instructions for the chain
func (h *PaymentHandler) getCryptoDepositInstructions(chain, asset string) []string {
	instructions := map[string][]string{
		"BITCOIN": {
			"Send BTC to the address above",
			"Minimum 1 confirmation required",
			"Network fee will be deducted from deposit",
			"DO NOT send from exchanges that don't support withdrawals",
		},
		"TRON": {
			fmt.Sprintf("Send %s (TRC20) to the address above", asset),
			"Minimum 19 confirmations required (~60 seconds)",
			"Make sure you're sending on TRON network (TRC20)",
			"DO NOT send ERC20 or other network tokens to this address",
		},
		"ETHEREUM": {
			fmt.Sprintf("Send %s (ERC20) to the address above", asset),
			"Minimum 12 confirmations required (~3 minutes)",
			"Gas fees will be paid by sender",
			"Make sure you're sending on Ethereum network (ERC20)",
		},
	}

	if inst, ok := instructions[strings.ToUpper(chain)]; ok {
		return inst
	}

	return []string{
		fmt.Sprintf("Send %s to the address above", asset),
		"Wait for network confirmations",
		"Funds will be credited automatically",
	}
}

// generateQRCodeURL generates a QR code URL for the deposit
func (h *PaymentHandler) generateQRCodeURL(address string, amount float64, asset string) string {
	baseURL := "https://chart.googleapis.com/chart?chs=300x300&cht=qr&chl="
	
	// Format based on asset
	var qrData string
	switch strings.ToUpper(asset) {
	case "BTC":
		//  Only include amount in QR if provided
		if amount > 0 {
			qrData = fmt.Sprintf("bitcoin:%s?amount=%.8f", address, amount)
		} else {
			qrData = fmt.Sprintf("bitcoin:%s", address)
		}
	case "ETH", "USDC":
		// Ethereum doesn't typically include amount in QR
		qrData = fmt.Sprintf("ethereum:%s", address)
	default:
		// For other assets, just the address
		qrData = address
	}

	return baseURL + qrData
}

// getBlockExplorerAddressURL returns block explorer URL for address
func (h *PaymentHandler) getBlockExplorerAddressURL(chain, address string) string {
	explorers := map[string]string{
		"BITCOIN":  "https://blockstream.info/address/",
		"TRON":     "https://tronscan.org/#/address/",
		"ETHEREUM": "https://etherscan.io/address/",
	}

	if baseURL, ok := explorers[strings.ToUpper(chain)]; ok {
		return baseURL + address
	}

	return ""
}

// getChainMemo returns memo/tag if required by chain
func (h *PaymentHandler) getChainMemo(chain string, wallet *cryptopb.Wallet) string {
	// Some chains like XRP, XLM require memo/tag
	// For now, most chains don't need it
	switch strings.ToUpper(chain) {
	case "XRP":
		// If XRP, return destination tag
		if wallet.Memo != "" {
			return wallet.Memo
		}
	case "XLM":
		// If Stellar, return memo
		if wallet.Memo != "" {
			return wallet.Memo
		}
	}

	return ""
}

// getMinimumDeposit returns minimum deposit amount for chain/asset
func (h *PaymentHandler) getMinimumDeposit(chain, asset string) float64 {
	minimums := map[string]map[string]float64{
		"BITCOIN": {
			"BTC": 0.0001, // 0.0001 BTC minimum
		},
		"TRON": {
			"TRX":  10.0,  // 10 TRX minimum
			"USDT": 1.0,   // 1 USDT minimum
		},
		"ETHEREUM": {
			"ETH":  0.001, // 0.001 ETH minimum
			"USDC": 1.0,   // 1 USDC minimum
		},
	}

	if chainMins, ok := minimums[strings.ToUpper(chain)]; ok {
		if min, ok := chainMins[strings.ToUpper(asset)]; ok {
			return min
		}
	}

	return 0
}

// getRequiredConfirmations returns required confirmations for chain
func (h *PaymentHandler) getRequiredConfirmations(chain string) int32 {
	confirmations := map[string]int32{
		"BITCOIN":  1,  // 1 confirmation for BTC
		"TRON":     19, // 19 confirmations for TRON (~60 seconds)
		"ETHEREUM": 12, // 12 confirmations for ETH (~3 minutes)
	}

	if confs, ok := confirmations[strings.ToUpper(chain)]; ok {
		return confs
	}

	return 1 // Default
}