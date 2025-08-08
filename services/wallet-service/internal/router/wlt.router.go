package router

import (
	"wallet-service/internal/handler"
	"wallet-service/internal/usecase/wallet"
	"x/shared/auth/middleware"

	"github.com/go-chi/chi/v5"
)

func New(uc *wallet.Service) *chi.Mux {
	auth := middleware.RequireAuth()
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)

		r.Route("/api", func(r chi.Router) {
			r.Post("/update-wallet", handler.UpdateBalanceHandler(uc))
			r.Get("/ws/wallet/{userID}", handler.WalletWSHandler(uc))
			r.Get("/wallet/summary/{userID}", handler.WalletSummaryHandler(uc))
			r.Get("/wallet/networth/{userID}", handler.NetworthHandler(uc))
			// other secured endpoints
		})
	})

	// r.Get("/ws/wallet/{userID}", handler.WalletWSHandler(uc))

	return r
}
