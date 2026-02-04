package main

import (
	"bufio"
	"context"
	"crypto-service/internal/chains/ethereum"
	"crypto-service/internal/config"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

var (
	ethChain        *ethereum.EthereumChain
	ctx             context.Context
	logger          *zap.Logger
	senderWallet    *domain.Wallet
	recipientWallet *domain.Wallet
	ethAsset        *domain.Asset
	usdcAsset       *domain.Asset
	cfg             *config.Config
	circleEnabled   bool
)

func main() {
	_ = godotenv.Load()

	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	ctx = context.Background()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║        ETHEREUM CRYPTO SERVICE - INTERACTIVE TEST             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if err := initialize(); err != nil {
		fmt.Printf("❌ Initialization failed: %v\n", err)
		return
	}

	runInteractiveTest()
}

func initialize() error {
	fmt.Println("⏳ Initializing Ethereum service...")

	var err error
	cfg, err = config.Load(logger)
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	// Initialize Ethereum chain with Circle support
	ethChain, err = ethereum.NewEthereumChain(
		cfg.Ethereum.RPCURL,
		cfg.Circle.APIKey,
		cfg.Circle.Environment,
		logger,
	)
	if err != nil {
		return fmt.Errorf("Ethereum init failed: %w", err)
	}

	// Check if Circle is enabled
	circleEnabled = cfg.Circle.Enabled && cfg.Circle.APIKey != ""

	fmt.Printf("✅ Connected to Ethereum %s network\n", cfg.Ethereum.Network)
	fmt.Printf("   Chain ID: %d\n", cfg.Ethereum.ChainID)
	fmt.Printf("   USDC Address: %s\n", cfg.Ethereum.USDCAddress)
	fmt.Printf("   Circle Enabled: %v\n\n", circleEnabled)

	ethAsset = &domain.Asset{
		Chain:    "ETHEREUM",
		Symbol:   "ETH",
		Type:     domain.AssetTypeNative,
		Decimals: 18,
	}

	usdcAsset = &domain.Asset{
		Chain:        "ETHEREUM",
		Symbol:       "USDC",
		ContractAddr: &cfg.Ethereum.USDCAddress,
		Type:         domain.AssetTypeToken,
		Decimals:     6,
	}

	return nil
}

func runInteractiveTest() {
	step1SetupWallets()
	waitForUser("Press ENTER to continue to balance check...")

	step2CheckBalances()
	waitForUser("If you need funds, get them now. Press ENTER when ready...")

	step3ChooseAssetAndSend()
	waitForUser("Press ENTER to check final balances...")

	step4CheckFinalBalances()

	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    ✅ TEST COMPLETED!                         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
}

// ============================================================================
// STEP 1: SETUP WALLETS
// ============================================================================

func step1SetupWallets() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 1: WALLET SETUP                                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Choose an option:")
	fmt.Println("  1. Generate new wallets (Standard Ethereum)")
	if circleEnabled {
		fmt.Println("  2. Generate new wallets (Circle USDC)")
	}
	fmt.Println("  3. Enter sender wallet manually")
	fmt.Println("  4. Load from eth_wallets.txt (if exists)")
	fmt.Println()

	choice := readInput("Enter choice: ")

	switch strings.TrimSpace(choice) {
	case "1":
		generateNewWallets("standard")
	case "2":
		if circleEnabled {
			generateNewWallets("circle")
		} else {
			fmt.Println("Circle not enabled, generating standard wallets...")
			generateNewWallets("standard")
		}
	case "3":
		enterWalletsManually()
	case "4":
		loadWalletsFromFile()
	default:
		fmt.Println("Invalid choice, using manual entry...")
		enterWalletsManually()
	}

	// Validate addresses
	fmt.Println("\n🔍 Validating addresses...")
	if err := ethChain.ValidateAddress(senderWallet.Address); err != nil {
		fmt.Printf("❌ Invalid sender address: %v\n", err)
		os.Exit(1)
	}
	if err := ethChain.ValidateAddress(recipientWallet.Address); err != nil {
		fmt.Printf("❌ Invalid recipient address: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Both addresses are valid!")
	fmt.Println()
}

func generateNewWallets(walletType string) {
	var err error

	// ✅ Prepare context based on wallet type
	var senderCtx, recipientCtx context.Context

	if walletType == "circle" && circleEnabled {
		fmt.Println("\n📝 Generating Circle wallets for USDC...")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		
		// Context for Circle wallets
		senderCtx = context.WithValue(ctx, "wallet_type", "circle")
		senderCtx = context.WithValue(senderCtx, "user_id", fmt.Sprintf("test-sender-%d", time.Now().Unix()))
		senderCtx = context.WithValue(senderCtx, "asset", "USDC")

		recipientCtx = context.WithValue(ctx, "wallet_type", "circle")
		recipientCtx = context.WithValue(recipientCtx, "user_id", fmt.Sprintf("test-recipient-%d", time.Now().Unix()))
		recipientCtx = context.WithValue(recipientCtx, "asset", "USDC")
	} else {
		fmt.Println("\n📝 Generating standard Ethereum wallets...")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		
		// Context for standard wallets
		senderCtx = context.WithValue(ctx, "wallet_type", "standard")
		recipientCtx = context.WithValue(ctx, "wallet_type", "standard")
	}

	// Generate sender wallet
	fmt.Println("Creating SENDER wallet...")
	senderWallet, err = ethChain.GenerateWallet(senderCtx)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Sender wallet created!")
	fmt.Printf("   Address:     %s\n", senderWallet.Address)
	if walletType == "circle" {
		fmt.Printf("   Wallet ID:   %s\n", senderWallet.PrivateKey)
		fmt.Println("   Type:        Circle USDC Wallet")
	} else {
		fmt.Printf("   Private Key: %s\n", senderWallet.PrivateKey)
		fmt.Println("   Type:        Standard Ethereum Wallet")
	}
	fmt.Println()

	// Generate recipient wallet
	fmt.Println("Creating RECIPIENT wallet...")
	recipientWallet, err = ethChain.GenerateWallet(recipientCtx)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Recipient wallet created!")
	fmt.Printf("   Address:     %s\n", recipientWallet.Address)
	if walletType == "circle" {
		fmt.Printf("   Wallet ID:   %s\n", recipientWallet.PrivateKey)
		fmt.Println("   Type:        Circle USDC Wallet")
	} else {
		fmt.Printf("   Private Key: %s\n", recipientWallet.PrivateKey)
		fmt.Println("   Type:        Standard Ethereum Wallet")
	}
	fmt.Println()

	saveWalletsToFile(walletType)
	fmt.Println("💾 Wallets saved to: eth_wallets.txt")
}

func enterWalletsManually() {
	fmt.Println("\n📝 Enter Sender Wallet Details:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	senderAddress := readInput("Sender Address (0x...): ")
	senderPrivateKey := readInput("Sender Private Key or Circle Wallet ID: ")

	senderWallet = &domain.Wallet{
		Address:    strings.TrimSpace(senderAddress),
		PrivateKey: strings.TrimPrefix(strings.TrimSpace(senderPrivateKey), "0x"),
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}

	fmt.Println("\n📝 Enter Recipient Address:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	recipientAddress := readInput("Recipient Address (0x...): ")

	recipientWallet = &domain.Wallet{
		Address:   strings.TrimSpace(recipientAddress),
		Chain:     "ETHEREUM",
		CreatedAt: time.Now(),
	}

	fmt.Println("\n✅ Wallets configured!")
	fmt.Printf("   From: %s\n", senderWallet.Address)
	fmt.Printf("   To:   %s\n", recipientWallet.Address)
}

func loadWalletsFromFile() {
	fmt.Println("\n📂 Loading wallets from eth_wallets.txt...")

	data, err := os.ReadFile("eth_wallets.txt")
	if err != nil {
		fmt.Printf("❌ Failed to read eth_wallets.txt: %v\n", err)
		fmt.Println("Falling back to manual entry...")
		enterWalletsManually()
		return
	}

	lines := strings.Split(string(data), "\n")
	
	var senderAddr, senderKey, recipientAddr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "Address:") && senderAddr == "" {
			senderAddr = strings.TrimSpace(strings.TrimPrefix(line, "Address:"))
		} else if (strings.HasPrefix(line, "Private Key:") || strings.HasPrefix(line, "Wallet ID:")) && senderKey == "" {
			senderKey = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Private Key:"), "Wallet ID:"))
		} else if strings.HasPrefix(line, "Address:") && senderAddr != "" && recipientAddr == "" {
			recipientAddr = strings.TrimSpace(strings.TrimPrefix(line, "Address:"))
		}
	}

	if senderAddr == "" || senderKey == "" || recipientAddr == "" {
		fmt.Println("❌ Could not parse eth_wallets.txt properly")
		fmt.Println("Falling back to manual entry...")
		enterWalletsManually()
		return
	}

	senderWallet = &domain.Wallet{
		Address:    senderAddr,
		PrivateKey: senderKey,
		Chain:      "ETHEREUM",
		CreatedAt:  time.Now(),
	}

	recipientWallet = &domain.Wallet{
		Address:   recipientAddr,
		Chain:     "ETHEREUM",
		CreatedAt:  time.Now(),
	}

	fmt.Println("✅ Wallets loaded successfully!")
	fmt.Printf("   Sender:     %s\n", senderWallet.Address)
	fmt.Printf("   Recipient: %s\n", recipientWallet.Address)
}

// ============================================================================
// STEP 2: CHECK BALANCES
// ============================================================================

func step2CheckBalances() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 2: CHECK CURRENT BALANCES                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📊 Sender Balances:")
	senderETH, senderUSDC := checkBalances(senderWallet.Address)

	fmt.Println()
	fmt.Println("📊 Recipient Balances:")
	checkBalances(recipientWallet.Address)

	// Check if we need funds
	minETH := big.NewInt(10000000000000000) // 0.01 ETH in wei
	if senderETH.Cmp(minETH) < 0 {
		fmt.Println("\n⚠️  Sender has insufficient ETH!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📋 TO GET TESTNET FUNDS:")
		
		switch cfg.Ethereum.Network {
		case "goerli":
			fmt.Println("🌊 Goerli Faucets:")
			fmt.Println("1. https://goerlifaucet.com/")
			fmt.Println("2. https://faucets.chain.link/goerli")
			fmt.Println("3. https://goerli-faucet.pk910.de/")
		case "sepolia":
			fmt.Println("🌊 Sepolia Faucets:")
			fmt.Println("1. https://sepoliafaucet.com/")
			fmt.Println("2. https://faucets.chain.link/sepolia")
			fmt.Println("3. https://sepolia-faucet.pk910.de/")
		}
		
		fmt.Printf("\n   Paste address: %s\n", senderWallet.Address)
		fmt.Println()
	} else {
		fmt.Println("\n Sender has sufficient ETH balance!")
	}

	if senderUSDC.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("\nℹ️  Sender has no USDC (test will only send ETH)")
	}
}

// ============================================================================
// STEP 3: CHOOSE ASSET AND SEND
// ============================================================================

func step3ChooseAssetAndSend() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 3: SEND TRANSACTION                                    ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Choose what to send:")
	fmt.Println("  1. Send ETH (native)")
	fmt.Println("  2. Send USDC (ERC-20 token)")
	fmt.Println()

	choice := readInput("Enter choice (1/2): ")

	switch strings.TrimSpace(choice) {
	case "1":
		sendETH()
	case "2":
		sendUSDC()
	default:
		fmt.Println("Invalid choice, sending ETH...")
		sendETH()
	}
}

func sendETH() {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💎 SENDING ETH")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get current balance
	fmt.Println("\n📊 Checking current balance...")
	ethBalance, err := ethChain.GetBalance(ctx, senderWallet.Address, ethAsset)
	if err != nil {
		fmt.Printf("❌ Failed to get balance: %v\n", err)
		return
	}

	humanBalance := weiToETH(ethBalance.Amount)
	fmt.Printf("   Available: %s ETH\n\n", humanBalance)

	// Ask for amount
	var amountFloat float64
	for {
		amountStr := readInput(fmt.Sprintf("Enter amount to send in ETH (max: %s): ", humanBalance))
		
		_, err := fmt.Sscanf(amountStr, "%f", &amountFloat)
		if err != nil || amountFloat <= 0 {
			fmt.Println("❌ Invalid amount")
			continue
		}

		maxAmount, _ := new(big.Float).SetString(humanBalance)
		maxFloat, _ := maxAmount.Float64()
		
		if amountFloat > maxFloat {
			fmt.Printf("❌ Amount exceeds balance\n")
			continue
		}

		// Reserve for gas
		if amountFloat >= maxFloat-0.001 {
			fmt.Println("⚠️  Reserve some ETH for gas (~0.001 ETH)")
			if !askYesNo("Continue anyway?") {
				continue
			}
		}

		break
	}

	sendAmount := ethToWei(amountFloat)

	// Estimate fee
	fmt.Println("\n⏳ Estimating gas fee...")
	feeEstimate, err := ethChain.EstimateFee(ctx, &domain.TransactionRequest{
		From:     senderWallet.Address,
		To:       recipientWallet.Address,
		Asset:    ethAsset,
		Amount:   sendAmount,
		Priority: domain.TxPriorityNormal,
	})

	if err != nil {
		fmt.Printf("⚠️  Could not estimate fee: %v\n", err)
		feeEstimate = &domain.Fee{
			Amount:   big.NewInt(21000000000000), // ~0.000021 ETH
			Currency: "ETH",
		}
	}

	estimatedFeeETH := weiToETH(feeEstimate.Amount)

	fmt.Printf("\n📤 Transaction Summary:\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   From:      %s\n", senderWallet.Address)
	fmt.Printf("   To:        %s\n", recipientWallet.Address)
	fmt.Printf("   Amount:    %f ETH\n", amountFloat)
	fmt.Printf("   Est Fee:   %s ETH\n", estimatedFeeETH)
	fmt.Printf("   Total:     %f ETH\n", amountFloat+mustParseFloat(estimatedFeeETH))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if !askYesNo("\nConfirm and send?") {
		fmt.Println("❌ Cancelled")
		return
	}

	fmt.Println("\n⏳ Sending transaction...")

	result, err := ethChain.Send(ctx, &domain.TransactionRequest{
		From:       senderWallet.Address,
		To:         recipientWallet.Address,
		Asset:      ethAsset,
		Amount:     sendAmount,
		PrivateKey: senderWallet.PrivateKey,
		Priority:   domain.TxPriorityNormal,
	})

	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		return
	}

	printTransactionResult(result, "ETH", amountFloat)
	saveTransactionToFile(result.TxHash, senderWallet.Address, recipientWallet.Address, fmt.Sprintf("%f ETH", amountFloat))
}

func sendUSDC() {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("💵 SENDING USDC (ERC-20)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Check USDC balance
	fmt.Println("\n📊 Checking USDC balance...")
	usdcBalance, err := ethChain.GetBalance(ctx, senderWallet.Address, usdcAsset)
	if err != nil {
		fmt.Printf("❌ Failed to get USDC balance: %v\n", err)
		return
	}

	humanBalance := usdcToHuman(usdcBalance.Amount)
	fmt.Printf("   Available: %s USDC\n\n", humanBalance)

	if usdcBalance.Amount.Cmp(big.NewInt(0)) == 0 {
		fmt.Println("❌ You have no USDC to send!")
		fmt.Println("\nℹ️  To get testnet USDC:")
		fmt.Println("   1. Get testnet ETH first (for gas)")
		fmt.Println("   2. Use a faucet or DEX to get USDC")
		return
	}

	// Ask for amount
	var amountFloat float64
	for {
		amountStr := readInput(fmt.Sprintf("Enter amount to send in USDC (max: %s): ", humanBalance))
		
		_, err := fmt.Sscanf(amountStr, "%f", &amountFloat)
		if err != nil || amountFloat <= 0 {
			fmt.Println("❌ Invalid amount")
			continue
		}

		maxAmount, _ := new(big.Float).SetString(humanBalance)
		maxFloat, _ := maxAmount.Float64()
		
		if amountFloat > maxFloat {
			fmt.Printf("❌ Amount exceeds balance\n")
			continue
		}

		break
	}

	sendAmount := humanToUSDC(amountFloat)

	// Estimate fee (in ETH)
	fmt.Println("\n⏳ Estimating gas fee...")
	feeEstimate, err := ethChain.EstimateFee(ctx, &domain.TransactionRequest{
		From:     senderWallet.Address,
		To:       recipientWallet.Address,
		Asset:    usdcAsset,
		Amount:   sendAmount,
		Priority: domain.TxPriorityNormal,
	})

	if err != nil {
		fmt.Printf("⚠️  Could not estimate fee: %v\n", err)
		feeEstimate = &domain.Fee{
			Amount:   big.NewInt(65000000000000), // ~0.000065 ETH
			Currency: "ETH",
		}
	}

	estimatedFeeETH := weiToETH(feeEstimate.Amount)

	fmt.Printf("\n📤 Transaction Summary:\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   From:      %s\n", senderWallet.Address)
	fmt.Printf("   To:        %s\n", recipientWallet.Address)
	fmt.Printf("   Amount:    %f USDC\n", amountFloat)
	fmt.Printf("   Gas Fee:   %s ETH\n", estimatedFeeETH)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if !askYesNo("\nConfirm and send?") {
		fmt.Println("❌ Cancelled")
		return
	}

	fmt.Println("\n⏳ Sending transaction...")

	result, err := ethChain.Send(ctx, &domain.TransactionRequest{
		From:       senderWallet.Address,
		To:         recipientWallet.Address,
		Asset:      usdcAsset,
		Amount:     sendAmount,
		PrivateKey: senderWallet.PrivateKey,
		Priority:   domain.TxPriorityNormal,
	})

	if err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		return
	}

	printTransactionResult(result, "USDC", amountFloat)
	saveTransactionToFile(result.TxHash, senderWallet.Address, recipientWallet.Address, fmt.Sprintf("%f USDC", amountFloat))
}

// ============================================================================
// STEP 4: CHECK FINAL BALANCES
// ============================================================================

func step4CheckFinalBalances() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 4: CHECK FINAL BALANCES                                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📊 Sender Balances:")
	checkBalances(senderWallet.Address)

	fmt.Println()
	fmt.Println("📊 Recipient Balances:")
	checkBalances(recipientWallet.Address)
	
	fmt.Println()
	fmt.Println("⏰ Note: Balance updates may take 12-30 seconds.")
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func checkBalances(address string) (*big.Int, *big.Int) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━���━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Check ETH
	ethBalance, err := ethChain.GetBalance(ctx, address, ethAsset)
	if err != nil {
		fmt.Printf("   ❌ ETH:    Error - %v\n", err)
		ethBalance = &domain.Balance{Amount: big.NewInt(0)}
	} else {
		fmt.Printf("   ETH:   %s ETH\n", weiToETH(ethBalance.Amount))
	}

	// Check USDC
	usdcBalance, err := ethChain.GetBalance(ctx, address, usdcAsset)
	if err != nil {
		fmt.Printf("   ❌ USDC:   Error - %v\n", err)
		usdcBalance = &domain.Balance{Amount: big.NewInt(0)}
	} else {
		fmt.Printf("   USDC:  %s USDC\n", usdcToHuman(usdcBalance.Amount))
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	return ethBalance.Amount, usdcBalance.Amount
}

func printTransactionResult(result *domain.TransactionResult, asset string, amount float64) {
	fmt.Println("\n Transaction sent successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   TX Hash:   %s\n", result.TxHash)
	fmt.Printf("   Status:    %s\n", result.Status)
	fmt.Printf("   Amount:    %f %s\n", amount, asset)
	fmt.Printf("   Fee:       %s ETH\n", weiToETH(result.Fee))
	fmt.Printf("   Time:      %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("🔍 View on Block Explorer:\n")
	fmt.Printf("   %s\n", getExplorerURL(result.TxHash))
	fmt.Println()
	fmt.Println("💾 Transaction saved to: eth_transactions.txt")
}

// Conversion helpers
func weiToETH(wei *big.Int) string {
	eth := new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		big.NewFloat(1e18),
	)
	return eth.Text('f', 18)
}

func ethToWei(eth float64) *big.Int {
	wei := new(big.Float).Mul(
		big.NewFloat(eth),
		big.NewFloat(1e18),
	)
	result, _ := wei.Int(nil)
	return result
}

func usdcToHuman(smallest *big.Int) string {
	usdc := new(big.Float).Quo(
		new(big.Float).SetInt(smallest),
		big.NewFloat(1e6),
	)
	return usdc.Text('f', 6)
}

func humanToUSDC(usdc float64) *big.Int {
	smallest := new(big.Float).Mul(
		big.NewFloat(usdc),
		big.NewFloat(1e6),
	)
	result, _ := smallest.Int(nil)
	return result
}

func mustParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func getExplorerURL(txHash string) string {
	switch cfg.Ethereum.Network {
	case "mainnet":
		return fmt.Sprintf("https://etherscan.io/tx/%s", txHash)
	case "goerli":
		return fmt.Sprintf("https://goerli.etherscan.io/tx/%s", txHash)
	case "sepolia":
		return fmt.Sprintf("https://sepolia.etherscan.io/tx/%s", txHash)
	default:
		return fmt.Sprintf("https://etherscan.io/tx/%s", txHash)
	}
}

func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func waitForUser(message string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("⏸  %s\n", message)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	fmt.Println()
}

func askYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/n): ", question)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
func saveWalletsToFile(walletType string) {
	file, err := os.Create("eth_wallets.txt")
	if err != nil {
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "ETHEREUM %s WALLETS\n", strings.ToUpper(cfg.Ethereum.Network))
	if walletType == "circle" {
		fmt.Fprintf(file, "Type: Circle USDC Wallets\n")
	} else {
		fmt.Fprintf(file, "Type: Standard Ethereum Wallets\n")
	}
	fmt.Fprintf(file, "==========================\n\n")
	
	fmt.Fprintf(file, "SENDER WALLET:\n")
	fmt.Fprintf(file, "Address:     %s\n", senderWallet.Address)
	if walletType == "circle" {
		fmt.Fprintf(file, "Wallet ID:   %s\n", senderWallet.PrivateKey)
		fmt.Fprintf(file, "Type:        Circle (USDC only)\n\n")
	} else {
		fmt.Fprintf(file, "Private Key: %s\n", senderWallet.PrivateKey)
		fmt.Fprintf(file, "Type:        Standard (ETH + ERC-20)\n\n")
	}
	
	fmt.Fprintf(file, "RECIPIENT WALLET:\n")
	fmt.Fprintf(file, "Address:     %s\n", recipientWallet.Address)
	if recipientWallet.PrivateKey != "" {
		if walletType == "circle" {
			fmt.Fprintf(file, "Wallet ID:   %s\n", recipientWallet.PrivateKey)
			fmt.Fprintf(file, "Type:        Circle (USDC only)\n\n")
		} else {
			fmt.Fprintf(file, "Private Key: %s\n", recipientWallet.PrivateKey)
			fmt.Fprintf(file, "Type:        Standard (ETH + ERC-20)\n\n")
		}
	}
	
	if cfg.Ethereum.Network == "goerli" {
		fmt.Fprintf(file, "\nGet Goerli testnet ETH:\n")
		fmt.Fprintf(file, "  - https://goerlifaucet.com/\n")
		fmt.Fprintf(file, "  - https://faucets.chain.link/goerli\n")
	} else if cfg.Ethereum.Network == "sepolia" {
		fmt.Fprintf(file, "\nGet Sepolia testnet ETH:\n")
		fmt.Fprintf(file, "  - https://sepoliafaucet.com/\n")
		fmt.Fprintf(file, "  - https://faucets.chain.link/sepolia\n")
	}

	if circleEnabled && walletType == "circle" {
		fmt.Fprintf(file, "\n📝 Circle Wallet Notes:\n")
		fmt.Fprintf(file, "  - These are Circle-managed wallets for USDC\n")
		fmt.Fprintf(file, "  - No gas fees for USDC transfers\n")
		fmt.Fprintf(file, "  - Can only send/receive USDC\n")
		fmt.Fprintf(file, "  - For ETH, generate a standard wallet\n")
	}
}

func saveTransactionToFile(txHash, from, to, amount string) {
	file, err := os.OpenFile("eth_transactions.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	explorerURL := getExplorerURL(txHash)

	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(file, "Transaction Hash: %s\n", txHash)
	fmt.Fprintf(file, "Network:          %s\n", cfg.Ethereum.Network)
	fmt.Fprintf(file, "From:             %s\n", from)
	fmt.Fprintf(file, "To:               %s\n", to)
	fmt.Fprintf(file, "Amount:           %s\n", amount)
	fmt.Fprintf(file, "Time:             %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Explorer:         %s\n", explorerURL)
	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n\n")
}