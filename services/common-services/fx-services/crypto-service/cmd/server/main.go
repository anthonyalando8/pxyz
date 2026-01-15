// cmd/server/main.go (FULL WORKING EXAMPLE)

package main

import (
	"context"
	"crypto-service/internal/chains/tron"
	"crypto-service/internal/config"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load config
	cfg, err := config.Load(logger)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// Initialize TRON chain
	tronChain, err := tron.NewTronChain(
		cfg.Tron.APIKey,
		cfg.Tron. Network,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to initialize TRON", zap. Error(err))
	}
	defer tronChain.Stop()

	ctx := context.Background()

	// ✅ Generate TWO wallets for testing
	fmt.Println("=== Generating Sender Wallet ===")
	senderWallet, err := tronChain.GenerateWallet(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Sender Address: %s\n", senderWallet.Address)
	fmt.Printf("Sender Private Key: %s\n\n", senderWallet.PrivateKey)

	fmt.Println("=== Generating Recipient Wallet ===")
	recipientWallet, err := tronChain. GenerateWallet(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Recipient Address: %s\n\n", recipientWallet.Address)

	// Get USDT contract
	usdtContract := tron.GetUSDTContract(cfg.Tron.Network)
	usdtAsset := &domain.Asset{
		Chain:        "TRON",
		Symbol:       "USDT",
		ContractAddr: &usdtContract,
		Type:         domain. AssetTypeToken,
		Decimals:     6,
	}

	// Check sender balance
	fmt.Println("=== Checking Sender USDT Balance ===")
	balance, err := tronChain. GetBalance(ctx, senderWallet.Address, usdtAsset)
	if err != nil {
		fmt.Printf("Error:  %v\n", err)
	} else {
		humanBalance := new(big.Float).Quo(
			new(big.Float).SetInt(balance.Amount),
			big.NewFloat(1000000),
		)
		fmt.Printf("Balance: %s USDT\n\n", humanBalance.String())
	}

	// Check TRX balance (needed for fees)
	fmt.Println("=== Checking Sender TRX Balance ===")
	trxAsset := &domain.Asset{
		Chain:    "TRON",
		Symbol:   "TRX",
		Type:     domain.AssetTypeNative,
		Decimals:  6,
	}
	
	trxBalance, err := tronChain.GetBalance(ctx, senderWallet.Address, trxAsset)
	if err != nil {
		fmt. Printf("Error: %v\n", err)
	} else {
		humanTRX := new(big.Float).Quo(
			new(big. Float).SetInt(trxBalance.Amount),
			big.NewFloat(1000000),
		)
		fmt.Printf("Balance: %s TRX\n\n", humanTRX. String())
	}

	// Instructions
	fmt.Println("=== Next Steps ===")
	fmt.Println("Before you can send USDT, you need:")
	fmt.Printf("1. Get test TRX (for fees): https://www.trongrid.io/shasta#%s\n", senderWallet.Address)
	fmt.Println("2. Get test USDT:")
	fmt.Println("   - Visit Shasta faucet or")
	fmt.Println("   - Use a testnet swap/exchange")
	fmt.Println()
	fmt.Println("After getting funds, you can send like this:")
	fmt.Printf("   From: %s\n", senderWallet.Address)
	fmt.Printf("   To:   %s\n", recipientWallet.Address)
	fmt.Printf("   Amount: 1 USDT\n")
	fmt.Println()

	// Example send (commented out - uncomment after getting test funds)
	/*
	if trxBalance.Amount.Cmp(big.NewInt(10000000)) > 0 && balance.Amount.Cmp(big.NewInt(1000000)) > 0 {
		fmt.Println("=== Sending 1 USDT ===")
		
		sendReq := &domain. TransactionRequest{
			From:        senderWallet.Address,
			To:         recipientWallet.Address,
			Asset:      usdtAsset,
			Amount:     big.NewInt(1000000), // 1 USDT
			PrivateKey: senderWallet.PrivateKey,
		}

		result, err := tronChain.Send(ctx, sendReq)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt. Printf("✅ Transaction Sent!\n")
			fmt.Printf("Hash: %s\n", result. TxHash)
			fmt.Printf("View:  https://shasta.tronscan.org/#/transaction/%s\n", result.TxHash)
		}
	} else {
		fmt.Println("⚠️ Insufficient balance to send")
		fmt.Printf("Need: 10+ TRX and 1+ USDT\n")
		fmt.Printf("Have: %s TRX, %s USDT\n", humanTRX.String(), humanBalance.String())
	}
	*/

	fmt.Println("✅ Crypto service initialized successfully!")
}