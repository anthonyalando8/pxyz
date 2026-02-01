// internal/domain/usdc.go
package domain

type contextKey string

const (
	// WalletTypeKey specifies the type of wallet to generate
	WalletTypeKey contextKey = "wallet_type"
	// UserIDKey for Circle wallet creation
	UserIDKey contextKey = "user_id"
	// AssetKey for asset info in context
	AssetKey contextKey = "asset"
	// ChainKey for chain info in context
	ChainKey contextKey = "chain"
)

// Wallet types
const (
	WalletTypeStandard = "standard" // Regular Ethereum wallet
	WalletTypeCircle   = "circle"   // Circle-managed USDC wallet
)