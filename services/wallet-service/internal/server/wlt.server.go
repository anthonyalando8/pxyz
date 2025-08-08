package server

import (
	"context"
	"net/http"
	"time"

	"wallet-service/internal/config"
	"wallet-service/internal/repository"
	"wallet-service/internal/router"
	"wallet-service/internal/usecase/wallet"
	"wallet-service/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer *http.Server
	db         *pgxpool.Pool
}

func New(cfg *config.Config) *Server {
	db := config.ConnectDB(cfg)

	repo := repository.NewWalletRepository(db)
	notifier := wallet. NewNotifier()
	uc := wallet.New(repo, &utils.DummyConverter{}, notifier)
	r := router.New(uc)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.HTTPAddr,
			Handler:      r,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		},
		db: db,
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	defer s.db.Close()
	return s.httpServer.Shutdown(ctx)
}