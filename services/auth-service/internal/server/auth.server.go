package server

import (
	"context"
	"log"
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/router"
	"auth-service/internal/usecase"
	"auth-service/internal/ws"
	"auth-service/pkg/jwtutil"
	"x/shared/auth/middleware"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(1) // Node ID 1 for this service
	
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	

	userUC := usecase.NewUserUsecase(userRepo, sf)

	jwtGen := jwtutil.LoadAndBuild(cfg.JWT)

	auth := middleware.RequireAuth()
	authHandler := handler.NewAuthHandler(userUC, jwtGen, auth)

	ws_server := ws.NewServer()
	ws_server.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ws.ListenAuthEvents(ctx, rdb, ws_server.Hub())
	
	wsHandler := handler.NewWSHandler(ws_server)


	r := chi.NewRouter()

	r = router.SetupRoutes(r, authHandler, auth, wsHandler, rdb).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}
