// --- notifier.go ---
package wallet

import (
	"encoding/json"
	"log"
	"sync"

	"wallet-service/internal/domain"
	"wallet-service/internal/usecase/common"

	"github.com/gorilla/websocket"
)

type Notifier struct {
	clients map[string]map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewNotifier() *Notifier {
	return &Notifier{
		clients: make(map[string]map[*websocket.Conn]bool),
	}
}

func (n *Notifier) RegisterConnection(userID string, conn *websocket.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.clients[userID] == nil {
		n.clients[userID] = make(map[*websocket.Conn]bool)
	}
	n.clients[userID][conn] = true
}

func (n *Notifier) UnregisterConnection(userID string, conn *websocket.Conn) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if conns, ok := n.clients[userID]; ok {
		delete(conns, conn)
		conn.Close()
		if len(conns) == 0 {
			delete(n.clients, userID)
		}
	}
}

func (n *Notifier) NotifyBalance(userID string, balance float64, wallet *domain.Wallet) {
	n.mu.Lock()
	defer n.mu.Unlock()

	message := common.WSMessage{
		Type: "networth_update",
		Data: map[string]interface{}{
			"user_id": userID,
			"balance": balance,
			"wallet": map[string]interface{}{
				"id":        wallet.ID,
				"currency":  wallet.Currency,
				"balance":   wallet.Balance,
				"available": wallet.Available,
				"locked":    wallet.Locked,
			},
		},
	}

	payload, _ := json.Marshal(message)

	for conn := range n.clients[userID] {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("Error sending networth to %s: %v", userID, err)
			conn.Close()
			delete(n.clients[userID], conn)
		}
	}
}

func (n *Notifier) NotifyInitial(userID string, wallets []*domain.Wallet, networth float64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	message := common.WSMessage{
		Type: "initial_data",
		Data: map[string]interface{}{
			"networth": networth,
			"wallets":  wallets,
		},
	}

	payload, _ := json.Marshal(message)

	for conn := range n.clients[userID] {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("Error sending initial data to %s: %v", userID, err)
			conn.Close()
			delete(n.clients[userID], conn)
		}
	}
}