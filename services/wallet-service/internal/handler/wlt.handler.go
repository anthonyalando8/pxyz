// internal/handler/update_balance.go
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"wallet-service/internal/usecase/wallet"
	"x/shared/response"
	"github.com/go-chi/chi/v5"
)

func UpdateBalanceHandler(walletUC *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type requestBody struct {
			UserID  string   `json:"user_id"`
			Balance *float64 `json:"balance"`
			Currency string  `json:"currency"`
		}

		var body requestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.Error(w, http.StatusBadRequest, "Invalid JSON body")
			return
		}

		if body.UserID == "" {
			response.Error(w, http.StatusBadRequest, "Missing user_id")
			return
		}
		if body.Balance == nil {
			response.Error(w, http.StatusBadRequest, "Missing balance")
			return
		}

		ctx := r.Context()
		err := walletUC.UpdateWalletBalance(ctx, body.UserID, body.Currency, *body.Balance)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to update balance")
			return
		}
		updated_wallet, err1 := walletUC.GetWalletByUserIDAndCurrency(ctx, body.UserID, body.Currency)
		if err1 != nil {
			response.Error(w, http.StatusInternalServerError, fmt.Sprintf("error fetching wallet: %v", err1))

			return
		}

		_, networth, err2 := walletUC.CalculateUserNetWorthInUSD(ctx, body.UserID)
		if err2 != nil {
			response.Error(w, http.StatusInternalServerError, fmt.Sprintf("error calculating net worth: %v", err2))
			return
		}

		walletUC.Notifier.NotifyBalance(body.UserID, networth, updated_wallet)

		w.Header().Set("Content-Type", "application/json")
		response.JSON(w, http.StatusOK, map[string]string{
			"message": "Balance updated successfully",
			"user_id": body.UserID,
			"balance": fmt.Sprintf("%f", *body.Balance),
		})
	}
}


func NetworthHandler(walletUC *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			response.Error(w, http.StatusBadRequest, "Missing user ID")
			return
		}
		ctx := r.Context()
		_,networth, err := walletUC.CalculateUserNetWorthInUSD(ctx, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to calculate net worth: %v", err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		response.JSON(w, http.StatusOK, map[string]float64{
			"net_worth": networth,
		})
	}
}

func WalletSummaryHandler(walletUC *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		if userID == "" {
			response.Error(w, http.StatusBadRequest, "Missing user ID")
			return
		}
		ctx := r.Context()
		wallets, networth, err := walletUC.CalculateUserNetWorthInUSD(ctx, userID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to calculate net worth: %v", err))
			return
		}	
		w.Header().Set("Content-Type", "application/json")
		response.JSON(w, http.StatusOK, map[string]interface{}{
			"wallets":   wallets,
			"net_worth": networth,
		})
	}
}
