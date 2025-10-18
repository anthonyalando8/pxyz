// internal/handler/ws_handler.go
package handler

import (
	// "encoding/json"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"wallet-service/internal/usecase/wallet"
	"x/shared/response"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func WalletWSHandler(walletUC *wallet.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := chi.URLParam(r, "userID")
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "WebSocket upgrade failed")
			return
		}

		walletUC.Notifier.RegisterConnection(userID, conn)
		defer walletUC.Notifier.UnregisterConnection(userID, conn)

		ctx := context.Background()
		// Load or create wallet for the user
		walletUC.CreateDefaultUserWallets(ctx, userID)
		// Notify initial balance and wallet info
		wallets, networth, err := walletUC.CalculateUserNetWorthInUSD(ctx, userID)
		if err == nil {
			walletUC.Notifier.NotifyInitial(userID, wallets, networth)
		} else {
			log.Printf("Error loading wallets: %v", err)
		}

		conn.SetPongHandler(func(appData string) error {
			return nil // Reset deadline or add log if needed
		})

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Client %s disconnected: %v", userID, err)
				break
			}

			if mt == websocket.TextMessage {
				var req struct {
					Action string `json:"action"`
				}
				if err := json.Unmarshal(msg, &req); err == nil && req.Action == "get_balance" {
					wallets, networth, _ := walletUC.CalculateUserNetWorthInUSD(ctx, userID)
					walletUC.Notifier.NotifyInitial(userID, wallets, networth)
				}
			}
		}
	}
}
