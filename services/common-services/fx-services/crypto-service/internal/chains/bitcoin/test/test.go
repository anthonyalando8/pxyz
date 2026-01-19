// cmd/test_btc/main.go
package main

import (
	"bufio"
	"context"
	"crypto-service/internal/chains/bitcoin"
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
	bitcoinChain    *bitcoin.BitcoinChain
	ctx             context.Context
	logger          *zap.Logger
	senderWallet    *domain.Wallet
	recipientWallet *domain.Wallet
	btcAsset        *domain.Asset
)

func main() {
	// Load . env
	_ = godotenv.Load()

	// Setup logger (simpler for CLI)
	logger, _ = zap.NewDevelopment()
	defer logger.Sync()

	ctx = context.Background()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║         BITCOIN CRYPTO SERVICE - INTERACTIVE TEST            ║")
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
	fmt.Println("⏳ Initializing Bitcoin service...")

	// Load config
	cfg, err := config. Load(logger)
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	// Initialize Bitcoin chain
	bitcoinChain, err = bitcoin.NewBitcoinChain(
		cfg.Bitcoin. RPCURL,
		cfg.Bitcoin.APIKey,
		cfg.Bitcoin. Network,
		logger,
	)
	if err != nil {
		return fmt.Errorf("Bitcoin init failed: %w", err)
	}

	fmt.Printf("✅ Connected to Bitcoin %s network\n\n", cfg.Bitcoin.Network)

	// Setup BTC asset
	btcAsset = &domain.Asset{
		Chain:    "BITCOIN",
		Symbol:   "BTC",
		Type:     domain.AssetTypeNative,
		Decimals: 8,
	}

	return nil
}

func runInteractiveTest() {
	defer bitcoinChain.Stop()

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
	fmt.Println("║                    ✅ TEST COMPLETED!                            ║")
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
	fmt.Println("  3. Load from btc_wallets.txt (if exists)")
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
	if err := bitcoinChain.ValidateAddress(senderWallet.Address); err != nil {
		fmt. Printf("❌ Invalid sender address: %v\n", err)
		os.Exit(1)
	}
	if err := bitcoinChain.ValidateAddress(recipientWallet.Address); err != nil {
		fmt.Printf("❌ Invalid recipient address: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Both addresses are valid!")
	fmt.Println()
}

func generateNewWallets() {
	fmt.Println("\n📝 Generating SENDER wallet...")
	var err error
	senderWallet, err = bitcoinChain.GenerateWallet(ctx)
	if err != nil {
		fmt.Printf("❌ Error:  %v\n", err)
		os.Exit(1)
	}

	fmt. Println("✅ Sender wallet created!")
	fmt.Printf("   Address:      %s\n", senderWallet.Address)
	fmt.Printf("   Private Key (WIF): %s\n", senderWallet.PrivateKey)
	fmt.Println()

	fmt.Println("📝 Generating RECIPIENT wallet...")
	recipientWallet, err = bitcoinChain.GenerateWallet(ctx)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Recipient wallet created!")
	fmt.Printf("   Address:     %s\n", recipientWallet.Address)
	fmt.Printf("   Private Key (WIF): %s\n", recipientWallet.PrivateKey)
	fmt.Println()

	saveWalletsToFile()
	fmt.Println("💾 Wallets saved to:  btc_wallets.txt")
}

func enterWalletsManually() {
	fmt.Println("\n📝 Enter Sender Wallet Details:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	senderAddress := readInput("Sender Address (Bitcoin address): ")
	senderPrivateKey := readInput("Sender Private Key (WIF format): ")

	senderWallet = &domain.Wallet{
		Address:    strings.TrimSpace(senderAddress),
		PrivateKey: strings.TrimSpace(senderPrivateKey),
		Chain:      "BITCOIN",
		CreatedAt:  time.Now(),
	}

	fmt. Println("\n📝 Enter Recipient Address:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	recipientAddress := readInput("Recipient Address (Bitcoin address): ")

	recipientWallet = &domain.Wallet{
		Address:   strings.TrimSpace(recipientAddress),
		Chain:     "BITCOIN",
		CreatedAt: time.Now(),
	}

	fmt.Println("\n✅ Wallets configured!")
	fmt.Printf("   From: %s\n", senderWallet.Address)
	fmt.Printf("   To:   %s\n", recipientWallet.Address)
}

func loadWalletsFromFile() {
	fmt.Println("\n📂 Loading wallets from btc_wallets.txt...")

	data, err := os.ReadFile("btc_wallets.txt")
	if err != nil {
		fmt.Printf("❌ Failed to read btc_wallets.txt: %v\n", err)
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
		} else if strings.HasPrefix(line, "Private Key:") && senderKey == "" {
			senderKey = strings.TrimSpace(strings.TrimPrefix(line, "Private Key:"))
		} else if strings.HasPrefix(line, "Address:") && senderAddr != "" && recipientAddr == "" {
			recipientAddr = strings.TrimSpace(strings.TrimPrefix(line, "Address:"))
		}
	}

	if senderAddr == "" || senderKey == "" || recipientAddr == "" {
		fmt.Println("❌ Could not parse btc_wallets.txt properly")
		fmt.Println("Falling back to manual entry...")
		enterWalletsManually()
		return
	}

	senderWallet = &domain. Wallet{
		Address:    senderAddr,
		PrivateKey: senderKey,
		Chain:      "BITCOIN",
		CreatedAt:  time. Now(),
	}

	recipientWallet = &domain. Wallet{
		Address:   recipientAddr,
		Chain:     "BITCOIN",
		CreatedAt: time.Now(),
	}

	fmt.Println("✅ Wallets loaded successfully!")
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
	senderBTCBalance := checkBalance(senderWallet.Address)

	fmt.Println()
	fmt.Println("📊 Recipient Balance:")
	checkBalance(recipientWallet.Address)

	// Check if we need funds (0.001 BTC minimum)
	minBTC := big.NewInt(100000) // 0.001 BTC in satoshis
	if senderBTCBalance. Cmp(minBTC) < 0 {
		fmt.Println("\n⚠️  Sender has insufficient BTC!")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("📋 TO GET TESTNET FUNDS:")
		fmt.Println("1. Visit: https://coinfaucet.eu/en/btc-testnet/")
		fmt.Println("   OR:  https://testnet-faucet.mempool.co/")
		fmt.Printf("2. Paste address: %s\n", senderWallet.Address)
		fmt.Println("3. Complete captcha and request testnet BTC")
		fmt.Println()
		fmt.Println("📱 Alternative faucets:")
		fmt.Println("   - https://bitcoinfaucet. uo1.net/")
		fmt.Println("   - https://testnet.help/en/btcfaucet/testnet")
		fmt.Println()
	} else {
		fmt.Println("\n✅ Sender has sufficient balance to send transactions!")
	}
}

// ============================================================================
// STEP 3: SEND TRANSACTION
// ============================================================================

func step3SendTransaction() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  STEP 3: SEND TRANSACTION                                    ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Get current sender balance first
	fmt.Println("📊 Checking current sender balance...")
	btcBalance, err := bitcoinChain.GetBalance(ctx, senderWallet.Address, btcAsset)
	if err != nil {
		fmt. Printf("❌ Failed to get balance: %v\n", err)
		return
	}

	// Convert to human-readable (BTC)
	humanBalance := new(big.Float).Quo(
		new(big.Float).SetInt(btcBalance. Amount),
		big.NewFloat(100000000), // 1 BTC = 100,000,000 satoshis
	)
	fmt.Printf("   Available: %s BTC\n\n", humanBalance.String())

	// Ask for amount to send
	var amountFloat float64
	for {
		amountStr := readInput(fmt.Sprintf("Enter amount to send in BTC (max:  %s): ", humanBalance.String()))
		
		_, err := fmt.Sscanf(amountStr, "%f", &amountFloat)
		if err != nil || amountFloat <= 0 {
			fmt.Println("❌ Invalid amount.  Please enter a positive number.")
			continue
		}

		// Check if amount exceeds balance
		maxAmount, _ := humanBalance.Float64()
		if amountFloat > maxAmount {
			fmt.Printf("❌ Amount exceeds balance.  You have %s BTC\n", humanBalance.String())
			continue
		}

		// Reserve some BTC for fees (~0.0001 BTC)
		if amountFloat >= maxAmount-0.0001 {
			fmt.Println("⚠️  Warning: You should reserve some BTC for transaction fees (~0.0001 BTC)")
			if !askYesNo("Continue anyway? ") {
				continue
			}
		}

		break
	}

	// Convert to satoshis (1 BTC = 100,000,000 satoshis)
	sendAmount := big.NewInt(int64(amountFloat * 100000000))
	humanAmount := fmt.Sprintf("%.8f BTC", amountFloat)

	fmt.Printf("\n📤 Transaction Summary:\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   From:    %s\n", senderWallet.Address)
	fmt.Printf("   To:      %s\n", recipientWallet.Address)
	fmt.Printf("   Amount:  %s\n", humanAmount)
	fmt.Printf("   Fee:     ~0.0001 BTC (estimated)\n")
	fmt. Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if !askYesNo("Confirm and send transaction?") {
		fmt.Println("❌ Transaction cancelled.")
		return
	}

	fmt.Println("\n⏳ Building and signing transaction...")

	sendReq := &domain.TransactionRequest{
		From:       senderWallet.Address,
		To:         recipientWallet.Address,
		Asset:      btcAsset,
		Amount:     sendAmount,
		PrivateKey: senderWallet. PrivateKey,
		Priority:   domain.TxPriorityNormal,
	}

	result, err := bitcoinChain. Send(ctx, sendReq)
	if err != nil {
		fmt.Printf("❌ Transaction failed: %v\n", err)
		return
	}

	fmt. Println("\n✅ Transaction sent successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("   TX Hash:   %s\n", result. TxHash)
	fmt.Printf("   Status:   %s\n", result.Status)
	fmt.Printf("   Fee:      %s BTC\n", formatSatoshis(result.Fee))
	fmt.Printf("   Time:     %s\n", result. Timestamp. Format("2006-01-02 15:04:05"))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("🔍 View on Block Explorer:\n")
	
	// Get correct explorer URL based on network
	explorerURL := getExplorerURL(result.TxHash)
	fmt.Printf("   %s\n", explorerURL)
	fmt.Println()

	// Save transaction details
	saveTransactionToFile(result. TxHash, senderWallet.Address, recipientWallet.Address, humanAmount)
	fmt.Println("💾 Transaction details saved to: btc_transactions.txt")
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
	
	fmt.Println()
	fmt.Println("⏰ Note: Balance updates may take a few minutes to appear.")
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func checkBalance(address string) *big.Int {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Check BTC balance
	btcBalance, err := bitcoinChain.GetBalance(ctx, address, btcAsset)
	if err != nil {
		fmt.Printf("   ❌ BTC:    Error - %v\n", err)
		return big.NewInt(0)
	}

	humanBTC := new(big.Float).Quo(
		new(big.Float).SetInt(btcBalance.Amount),
		big.NewFloat(100000000),
	)
	
	fmt.Printf("   BTC:  %s BTC (%s satoshis)\n", 
		humanBTC.String(), 
		btcBalance.Amount. String())

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	return btcBalance. Amount
}

func formatSatoshis(sats *big.Int) string {
	btc := new(big.Float).Quo(
		new(big.Float).SetInt(sats),
		big.NewFloat(100000000),
	)
	return btc.Text('f', 8)
}

func getExplorerURL(txHash string) string {
	// Determine if testnet or mainnet from config
	network := os.Getenv("BTC_NETWORK")
	
	if network == "testnet" {
		return fmt.Sprintf("https://blockstream.info/testnet/tx/%s", txHash)
	}
	return fmt.Sprintf("https://blockstream.info/tx/%s", txHash)
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
	response = strings. TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func saveWalletsToFile() {
	file, err := os.Create("btc_wallets.txt")
	if err != nil {
		return
	}
	defer file. Close()

	network := os.Getenv("BTC_NETWORK")
	
	fmt.Fprintf(file, "BITCOIN %s WALLETS\n", strings.ToUpper(network))
	fmt.Fprintf(file, "==========================\n\n")
	fmt.Fprintf(file, "SENDER WALLET:\n")
	fmt.Fprintf(file, "Address:     %s\n", senderWallet.Address)
	fmt.Fprintf(file, "Private Key:  %s\n\n", senderWallet. PrivateKey)
	fmt.Fprintf(file, "RECIPIENT WALLET:\n")
	fmt.Fprintf(file, "Address:     %s\n", recipientWallet.Address)
	if recipientWallet.PrivateKey != "" {
		fmt. Fprintf(file, "Private Key:  %s\n\n", recipientWallet.PrivateKey)
	}
	
	if network == "testnet" {
		fmt.Fprintf(file, "\nGet testnet BTC:\n")
		fmt.Fprintf(file, "  - https://coinfaucet.eu/en/btc-testnet/\n")
		fmt.Fprintf(file, "  - https://testnet-faucet.mempool.co/\n")
		fmt.Fprintf(file, "  - https://bitcoinfaucet.uo1.net/\n")
	}
}

func saveTransactionToFile(txHash, from, to, amount string) {
	file, err := os.OpenFile("btc_transactions.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	network := os.Getenv("BTC_NETWORK")
	explorerURL := getExplorerURL(txHash)

	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(file, "Transaction Hash: %s\n", txHash)
	fmt.Fprintf(file, "Network:          %s\n", network)
	fmt.Fprintf(file, "From:            %s\n", from)
	fmt.Fprintf(file, "To:              %s\n", to)
	fmt.Fprintf(file, "Amount:          %s\n", amount)
	fmt.Fprintf(file, "Time:            %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Explorer:         %s\n", explorerURL)
	fmt.Fprintf(file, "═══════════════════════════════════════════════════════════════\n\n")
}