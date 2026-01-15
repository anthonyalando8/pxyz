// cmd/test/main.go (UPDATED - Load existing or enter manually)

package main

import (
	"bufio"
	"context"
	"crypto-service/internal/chains/tron"
	"crypto-service/internal/config"
	"crypto-service/internal/domain"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

var (
	tronChain       *tron.TronChain
	ctx             context.Context
	logger          *zap.Logger
	senderWallet    *domain. Wallet
	recipientWallet *domain.Wallet
	usdtAsset       *domain.Asset
	trxAsset        *domain.Asset
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Setup logger (simpler for CLI)
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	ctx = context.Background()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          TRON CRYPTO SERVICE - INTERACTIVE TEST              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Initialize
	if err := initialize(); err != nil {
		fmt.Printf("❌ Initialization failed: %v\n", err)
		return
	}

	// Run step-by-step flow
	runInteractiveTest()
}

func initialize() error {
	fmt.Println("⏳ Initializing TRON service...")

	// Load config
	cfg, err := config.Load(logger)
	if err != nil {
		return fmt. Errorf("config load failed: %w", err)
	}

	// Initialize TRON chain
	tronChain, err = tron.NewTronChain(
		cfg.Tron.APIKey,
		cfg.Tron.Network,
		logger,
	)
	if err != nil {
		return fmt. Errorf("TRON init failed: %w", err)
	}

	fmt.Printf("✅ Connected to TRON %s network\n\n", cfg.Tron. Network)

	// Setup assets
	usdtContract := tron.GetUSDTContract(cfg.Tron.Network)
	usdtAsset = &domain. Asset{
		Chain:        "TRON",
		Symbol:       "USDT",
		ContractAddr: &usdtContract,
		Type:         domain.AssetTypeToken,
		Decimals:     6,
	}

	trxAsset = &domain.Asset{
		Chain:    "TRON",
		Symbol:   "TRX",
		Type:     domain.AssetTypeNative,
		Decimals: 6,
	}

	return nil
}

func runInteractiveTest() {
	defer tronChain.Stop()

	// Step 1: Setup Wallets (Load or Generate)
	step1SetupWallets()
	waitForUser("Press ENTER to continue to balance check...")

	// Step 2: Check Balances
	step2CheckBalances()
	waitForUser("If you need funds, get them now.  Press ENTER when ready to send...")

	// Step 3: Send Transaction
	step3SendTransaction()
	waitForUser("Press ENTER to check final balances...")

	// Step 4: Check Final Balances
	step4CheckFinalBalances()

	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    ✅ TEST COMPLETED!                           ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
}

// ============================================================================
// STEP 1: SETUP WALLETS (Load existing or generate new or enter manually)
// ============================================================================

func step1SetupWallets() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 1: WALLET SETUP                                        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Choose an option:")
	fmt.Println("  1. Generate new wallets")
	fmt.Println("  2. Enter sender wallet manually")
	fmt.Println("  3. Load from wallets.txt (if exists)")
	fmt.Println()

	choice := readInput("Enter choice (1/2/3): ")

	switch strings.TrimSpace(choice) {
	case "1":
		generateNewWallets()
	case "2":
		enterWalletsManually()
	case "3":
		loadWalletsFromFile()
	default:
		fmt.Println("Invalid choice, using manual entry...")
		enterWalletsManually()
	}

	// Validate addresses
	fmt.Println("\n🔍 Validating addresses...")
	if err := tronChain.ValidateAddress(senderWallet.Address); err != nil {
		fmt. Printf("❌ Invalid sender address: %v\n", err)
		os.Exit(1)
	}
	if err := tronChain.ValidateAddress(recipientWallet.Address); err != nil {
		fmt.Printf("❌ Invalid recipient address: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Both addresses are valid!")
	fmt.Println()
}

func generateNewWallets() {
	fmt.Println("\n📝 Generating SENDER wallet...")
	var err error
	senderWallet, err = tronChain.GenerateWallet(ctx)
	if err != nil {
		fmt.Printf("❌ Error:  %v\n", err)
		os.Exit(1)
	}

	fmt. Println("✅ Sender wallet created!")
	fmt.Printf("   Address:     %s\n", senderWallet.Address)
	fmt.Printf("   Private Key: %s\n", senderWallet. PrivateKey)
	fmt.Println()

	fmt.Println("📝 Generating RECIPIENT wallet...")
	recipientWallet, err = tronChain.GenerateWallet(ctx)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Recipient wallet created!")
	fmt.Printf("   Address:     %s\n", recipientWallet.Address)
	fmt.Printf("   Private Key: %s\n", recipientWallet. PrivateKey)
	fmt.Println()

	saveWalletsToFile()
	fmt.Println("💾 Wallets saved to:  wallets.txt")
}

func enterWalletsManually() {
	fmt.Println("\n📝 Enter Sender Wallet Details:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	senderAddress := readInput("Sender Address (34 chars, starts with T): ")
	senderPrivateKey := readInput("Sender Private Key (64 hex chars): ")

	senderWallet = &domain.Wallet{
		Address:    strings.TrimSpace(senderAddress),
		PrivateKey: strings.TrimSpace(senderPrivateKey),
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}

	fmt. Println("\n📝 Enter Recipient Address:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	recipientAddress := readInput("Recipient Address (34 chars, starts with T): ")

	recipientWallet = &domain.Wallet{
		Address:   strings.TrimSpace(recipientAddress),
		Chain:     "TRON",
		CreatedAt: time.Now(),
	}

	fmt.Println("\n✅ Wallets configured!")
	fmt.Printf("   From:  %s\n", senderWallet.Address)
	fmt.Printf("   To:    %s\n", recipientWallet.Address)
}

func loadWalletsFromFile() {
	fmt.Println("\n📂 Loading wallets from wallets. txt...")

	data, err := os.ReadFile("wallets.txt")
	if err != nil {
		fmt. Printf("❌ Failed to read wallets.txt: %v\n", err)
		fmt.Println("Falling back to manual entry...")
		enterWalletsManually()
		return
	}

	lines := strings.Split(string(data), "\n")
	
	var senderAddr, senderKey, recipientAddr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "Address:") && senderAddr == "" {
			senderAddr = strings.TrimSpace(strings.TrimPrefix(line, "Address: "))
		} else if strings.HasPrefix(line, "Private Key:") && senderKey == "" {
			senderKey = strings.TrimSpace(strings. TrimPrefix(line, "Private Key:"))
		} else if strings.HasPrefix(line, "Address:") && senderAddr != "" && recipientAddr == "" {
			recipientAddr = strings.TrimSpace(strings.TrimPrefix(line, "Address:"))
		}
	}

	if senderAddr == "" || senderKey == "" || recipientAddr == "" {
		fmt. Println("❌ Could not parse wallets.txt properly")
		fmt.Println("Falling back to manual entry...")
		enterWalletsManually()
		return
	}

	senderWallet = &domain. Wallet{
		Address:    senderAddr,
		PrivateKey: senderKey,
		Chain:      "TRON",
		CreatedAt:  time.Now(),
	}

	recipientWallet = &domain. Wallet{
		Address:   recipientAddr,
		Chain:      "TRON",
		CreatedAt: time.Now(),
	}

	fmt. Println("✅ Wallets loaded successfully!")
	fmt.Printf("   Sender:     %s\n", senderWallet.Address)
	fmt.Printf("   Recipient: %s\n", recipientWallet. Address)
}

// ============================================================================
// STEP 2: CHECK BALANCES
// ============================================================================

func step2CheckBalances() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 2: CHECK CURRENT BALANCES                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📊 Sender Balance:")
	senderTRXBalance := checkBalance(senderWallet.Address)

	fmt.Println()
	fmt.Println("📊 Recipient Balance:")
	checkBalance(recipientWallet.Address)

	// Check if we need funds
	minTRX := big.NewInt(1000000) // 1 TRX
	if senderTRXBalance. Cmp(minTRX) < 0 {
		fmt.Println("\n⚠️  Sender has insufficient TRX!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📋 TO GET TEST FUNDS:")
		fmt.Printf("1. Visit: https://www.trongrid.io/shasta\n")
		fmt.Printf("2. Paste address: %s\n", senderWallet.Address)
		fmt.Println("3. Click 'Submit' to get 10,000 test TRX")
		fmt.Println()
		fmt.Printf("📱 Direct link: https://www.trongrid.io/shasta#%s\n", senderWallet. Address)
		fmt.Println()
	} else {
		fmt.Println("\n✅ Sender has sufficient balance to send transactions!")
	}
}

// ============================================================================
// STEP 3: SEND TRANSACTION
// ============================================================================

// cmd/test/main.go (UPDATE step3SendTransaction)

// ============================================================================
// STEP 3: SEND TRANSACTION
// ============================================================================

func step3SendTransaction() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 3: SEND TRANSACTION                                    ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Get current sender balance first
	fmt. Println("📊 Checking current sender balance...")
	trxBalance, err := tronChain.GetBalance(ctx, senderWallet. Address, trxAsset)
	if err != nil {
		fmt.Printf("❌ Failed to get balance: %v\n", err)
		return
	}

	// Convert to human-readable
	humanBalance := new(big.Float).Quo(
		new(big.Float).SetInt(trxBalance. Amount),
		big.NewFloat(1000000),
	)
	fmt.Printf("   Available:  %s TRX\n\n", humanBalance.String())

	// ✅ Ask for amount to send
	var amountFloat float64
	for {
		amountStr := readInput(fmt.Sprintf("Enter amount to send in TRX (max:  %s): ", humanBalance.String()))
		
		_, err := fmt.Sscanf(amountStr, "%f", &amountFloat)
		if err != nil || amountFloat <= 0 {
			fmt. Println("❌ Invalid amount. Please enter a positive number.")
			continue
		}

		// Check if amount exceeds balance
		maxAmount, _ := humanBalance.Float64()
		if amountFloat > maxAmount {
			fmt. Printf("❌ Amount exceeds balance. You have %s TRX\n", humanBalance.String())
			continue
		}

		// Reserve some TRX for fees (0.1 TRX)
		if amountFloat >= maxAmount-0.1 {
			fmt. Println("⚠️  Warning: You should keep some TRX for transaction fees (~0.1 TRX)")
			if !askYesNo("Continue anyway?") {
				continue
			}
		}

		break
	}

	// Convert to SUN (1 TRX = 1,000,000 SUN)
	sendAmount := big.NewInt(int64(amountFloat * 1000000))
	humanAmount := fmt.Sprintf("%.6f TRX", amountFloat)

	fmt.Printf("\n📤 Transaction Summary:\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   From:     %s\n", senderWallet.Address)
	fmt.Printf("   To:      %s\n", recipientWallet.Address)
	fmt.Printf("   Amount:  %s\n", humanAmount)
	fmt.Printf("   Fee:     ~0.1 TRX (estimated)\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt. Println()

	if !askYesNo("Confirm and send transaction?") {
		fmt.Println("❌ Transaction cancelled.")
		return
	}

	fmt. Println("\n⏳ Building and signing transaction...")

	sendReq := &domain.TransactionRequest{
		From:       senderWallet.Address,
		To:         recipientWallet.Address,
		Asset:      trxAsset,
		Amount:     sendAmount,
		PrivateKey: senderWallet.PrivateKey,
		Priority:   domain.TxPriorityNormal,
	}

	result, err := tronChain. Send(ctx, sendReq)
	if err != nil {
		fmt.Printf("❌ Transaction failed: %v\n", err)
		return
	}

	fmt. Println("\n✅ Transaction sent successfully!")
	fmt. Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   TX Hash:   %s\n", result. TxHash)
	fmt.Printf("   Status:   %s\n", result.Status)
	fmt.Printf("   Time:     %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("🔍 View on Block Explorer:\n")
	fmt.Printf("   https://shasta.tronscan.org/#/transaction/%s\n", result.TxHash)
	fmt.Println()

	// Save transaction details
	saveTransactionToFile(result. TxHash, senderWallet.Address, recipientWallet.Address, humanAmount)
	fmt.Println("💾 Transaction details saved to:  transactions. txt")
}

// ============================================================================
// STEP 4: CHECK FINAL BALANCES
// ============================================================================

func step4CheckFinalBalances() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 4: CHECK FINAL BALANCES                                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📊 Sender Balance:")
	checkBalance(senderWallet.Address)

	fmt.Println()
	fmt.Println("📊 Recipient Balance:")
	checkBalance(recipientWallet. Address)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// cmd/test/main.go (UPDATE checkBalance)

func checkBalance(address string) *big.Int {
	fmt. Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Check TRX
	trxBalance, err := tronChain.GetBalance(ctx, address, trxAsset)
	if err != nil {
		fmt.Printf("   ❌ TRX:    Error - %v\n", err)
		return big.NewInt(0)
	}

	humanTRX := new(big.Float).Quo(
		new(big.Float).SetInt(trxBalance.Amount),
		big.NewFloat(1000000),
	)
	fmt.Printf("   TRX:  %s TRX\n", humanTRX.String())

	// Check USDT
	usdtBalance, err := tronChain. GetBalance(ctx, address, usdtAsset)
	if err == nil {
		humanUSDT := new(big. Float).Quo(
			new(big.Float).SetInt(usdtBalance.Amount),
			big.NewFloat(1000000),
		)
		if usdtBalance.Amount.Cmp(big.NewInt(0)) > 0 {
			fmt. Printf("   USDT:  %s USDT\n", humanUSDT.String())
		} else {
			fmt.Printf("   USDT: 0 USDT\n")
		}
	} else {
		fmt.Printf("   USDT: 0 USDT\n")
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	return trxBalance. Amount
}

func readInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func waitForUser(message string) {
	fmt. Println()
	fmt. Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("⏸  %s\n", message)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
	fmt.Println()
}

func askYesNo(question string) bool {
	reader := bufio. NewReader(os.Stdin)
	fmt.Printf("%s (y/n): ", question)
	response, _ := reader.ReadString('\n')
	response = strings. TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func saveWalletsToFile() {
	file, err := os.Create("wallets.txt")
	if err != nil {
		return
	}
	defer file. Close()

	fmt.Fprintf(file, "TRON TEST WALLETS\n")
	fmt.Fprintf(file, "=================\n\n")
	fmt.Fprintf(file, "SENDER WALLET:\n")
	fmt.Fprintf(file, "Address:      %s\n", senderWallet.Address)
	fmt.Fprintf(file, "Private Key:  %s\n\n", senderWallet. PrivateKey)
	fmt.Fprintf(file, "RECIPIENT WALLET:\n")
	fmt.Fprintf(file, "Address:     %s\n", recipientWallet.Address)
	if recipientWallet.PrivateKey != "" {
		fmt. Fprintf(file, "Private Key: %s\n\n", recipientWallet.PrivateKey)
	}
	fmt.Fprintf(file, "\nGet test TRX:  https://www.trongrid.io/shasta#%s\n", senderWallet.Address)
}

func saveTransactionToFile(txHash, from, to, amount string) {
	file, err := os.OpenFile("transactions.txt", os. O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(file, "Transaction Hash: %s\n", txHash)
	fmt.Fprintf(file, "From:            %s\n", from)
	fmt.Fprintf(file, "To:              %s\n", to)
	fmt.Fprintf(file, "Amount:          %s\n", amount)
	fmt.Fprintf(file, "Time:            %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Explorer:        https://shasta.tronscan.org/#/transaction/%s\n", txHash)
	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n\n")
}