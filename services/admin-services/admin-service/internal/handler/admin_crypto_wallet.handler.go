// internal/handler/admin_crypto_wallet_handler.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	cryptopb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
)

// ============================================================================
// WALLET MANAGEMENT
// ============================================================================

// CreateWallets creates multiple wallets for a user (batch operation)
// POST /admin/crypto/wallets/batch
func (h *AdminHandler) CreateWallets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		UserID  string `json:"user_id"`
		Wallets []struct {
			Chain string `json:"chain"`
			Asset string `json:"asset"`
			Label string `json:"label,omitempty"`
		} `json:"wallets"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.UserID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}
	if len(req.Wallets) == 0 {
		h.respondError(w, http.StatusBadRequest, "wallets array cannot be empty", nil)
		return
	}

	// Convert to protobuf
	walletSpecs := make([]*cryptopb.WalletSpec, len(req.Wallets))
	for i, w := range req.Wallets {
		walletSpecs[i] = &cryptopb.WalletSpec{
			Chain: stringToChainEnum(w.Chain),
			Asset: w.Asset,
			Label: w.Label,
		}
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.CreateWallets(ctx, &cryptopb.CreateWalletsRequest{
		UserId:  req.UserID,
		Wallets: walletSpecs,
	})
	if err != nil {
		h.logger.Error("Failed to create wallets",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to create wallets", err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallets":       resp.Wallets,
			"errors":        resp.Errors,
			"success_count": resp.SuccessCount,
			"failed_count":  resp.FailedCount,
			"message":       resp.Message,
		},
	})
}

// InitializeUserWallets creates all supported wallets for a user
// POST /admin/crypto/wallets/initialize
func (h *AdminHandler) InitializeUserWallets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		UserID       string   `json:"user_id"`
		Chains       []string `json:"chains,omitempty"`        // Optional: specific chains
		Assets       []string `json:"assets,omitempty"`        // Optional: specific assets
		SkipExisting bool     `json:"skip_existing,omitempty"` // Default: false
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate
	if req.UserID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Convert chains
	chains := make([]cryptopb.Chain, len(req.Chains))
	for i, c := range req.Chains {
		chains[i] = stringToChainEnum(c)
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.InitializeUserWallets(ctx, &cryptopb.InitializeUserWalletsRequest{
		UserId:       req.UserID,
		Chains:       chains,
		Assets:       req.Assets,
		SkipExisting: req.SkipExisting,
	})
	if err != nil {
		h.logger.Error("Failed to initialize wallets",
			zap.String("user_id", req.UserID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to initialize wallets", err)
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallets":       resp.Wallets,
			"errors":        resp.Errors,
			"total_created": resp.TotalCreated,
			"total_skipped": resp.TotalSkipped,
			"total_failed":  resp.TotalFailed,
			"message":       resp.Message,
		},
	})
}

// GetUserWallets retrieves all wallets for a user
// GET /admin/crypto/wallets/user/{user_id}
func (h *AdminHandler) GetUserWallets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user_id from URL
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	activeOnly := r.URL.Query().Get("active_only") == "true"

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.GetUserWallets(ctx, &cryptopb.GetUserWalletsRequest{
		UserId:     userID,
		Chain:      stringToChainEnum(chain),
		Asset:      asset,
		ActiveOnly: activeOnly,
	})
	if err != nil {
		h.logger.Error("Failed to get user wallets",
			zap.String("user_id", userID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get wallets", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallets": resp.Wallets,
			"total":   resp.Total,
		},
	})
}

// GetWalletByAddress retrieves wallet by blockchain address
// GET /admin/crypto/wallets/address/{address}
func (h *AdminHandler) GetWalletByAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	address := r.URL.Query().Get("address")
	if address == "" {
		h.respondError(w, http.StatusBadRequest, "address is required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.GetWalletByAddress(ctx, &cryptopb.GetWalletByAddressRequest{
		Address: address,
	})
	if err != nil {
		h.logger.Error("Failed to get wallet by address",
			zap.String("address", address),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "Wallet not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallet": resp.Wallet,
		},
	})
}

// RefreshBalance forces a balance refresh from blockchain
// POST /admin/crypto/wallets/{wallet_id}/refresh
func (h *AdminHandler) RefreshBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	walletIDStr := r.URL.Query().Get("wallet_id")
	if walletIDStr == "" {
		h.respondError(w, http.StatusBadRequest, "wallet_id is required", nil)
		return
	}

	walletID, err := strconv.ParseInt(walletIDStr, 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid wallet_id", err)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.respondError(w, http.StatusBadRequest, "user_id is required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.RefreshBalance(ctx, &cryptopb.RefreshBalanceRequest{
		WalletId: walletID,
		UserId:   userID,
	})
	if err != nil {
		h.logger.Error("Failed to refresh balance",
			zap.Int64("wallet_id", walletID),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to refresh balance", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"balance":          resp.Balance,
			"previous_balance": resp.PreviousBalance,
			"updated_at":       resp.UpdatedAt,
		},
	})
}

// ============================================================================
// SYSTEM WALLET MANAGEMENT (Admin Only)
// ============================================================================

// GetSystemWallets retrieves all system hot wallets
// GET /admin/crypto/system/wallets
func (h *AdminHandler) GetSystemWallets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Optional filters
	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	activeOnly := r.URL.Query().Get("active_only") == "true"

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.GetSystemWallets(ctx, &cryptopb.GetSystemWalletsRequest{
		Chain:      stringToChainEnum(chain),
		Asset:      asset,
		ActiveOnly: activeOnly,
	})
	if err != nil {
		h.logger.Error("Failed to get system wallets", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get system wallets", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallets": resp.Wallets,
			"total":   resp.Total,
			"summary": resp.Summary,
		},
	})
}

// GetSystemBalance gets balance for specific system wallet
// GET /admin/crypto/system/balance
func (h *AdminHandler) GetSystemBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")
	forceRefresh := r.URL.Query().Get("force_refresh") == "true"

	if chain == "" || asset == "" {
		h.respondError(w, http.StatusBadRequest, "chain and asset are required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.GetSystemBalance(ctx, &cryptopb.GetSystemBalanceRequest{
		Chain:        stringToChainEnum(chain),
		Asset:        asset,
		ForceRefresh: forceRefresh,
	})
	if err != nil {
		h.logger.Error("Failed to get system balance",
			zap.String("chain", chain),
			zap.String("asset", asset),
			zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "Failed to get system balance", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallet":             resp.Wallet,
			"blockchain_balance": resp.BlockchainBalance,
			"cached_balance":     resp.CachedBalance,
			"balances_match":     resp.BalancesMatch,
			"updated_at":         resp.UpdatedAt,
		},
	})
}

// GetSystemWalletByAsset gets specific system wallet
// GET /admin/crypto/system/wallet
func (h *AdminHandler) GetSystemWalletByAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chain := r.URL.Query().Get("chain")
	asset := r.URL.Query().Get("asset")

	if chain == "" || asset == "" {
		h.respondError(w, http.StatusBadRequest, "chain and asset are required", nil)
		return
	}

	// Call gRPC service
	resp, err := h.cryptoClient.WalletClient.GetSystemWalletByAsset(ctx, &cryptopb.GetSystemWalletByAssetRequest{
		Chain: stringToChainEnum(chain),
		Asset: asset,
	})
	if err != nil {
		h.logger.Error("Failed to get system wallet",
			zap.String("chain", chain),
			zap.String("asset", asset),
			zap.Error(err))
		h.respondError(w, http.StatusNotFound, "System wallet not found", err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"wallet": resp.Wallet,
		},
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func stringToChainEnum(chain string) cryptopb.Chain {
	if chain == "" {
		return cryptopb.Chain_CHAIN_UNSPECIFIED
	}

	switch chain {
	case "TRON":
		return cryptopb.Chain_CHAIN_TRON
	case "BITCOIN":
		return cryptopb.Chain_CHAIN_BITCOIN
	case "ETHEREUM":
		return cryptopb.Chain_CHAIN_ETHEREUM
	default:
		return cryptopb.Chain_CHAIN_UNSPECIFIED
	}
}

// respondJSON sends JSON response
func (h *AdminHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends error response
func (h *AdminHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message,
		zap.Error(err),
		zap.Int("status", status))

	response := map[string]interface{}{
		"success": false,
		"error":   message,
	}

	if err != nil {
		response["details"] = err.Error()
	}

	h.respondJSON(w, status, response)
}