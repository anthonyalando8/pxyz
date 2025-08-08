package server

import (
	"log"
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/router"
	"auth-service/internal/usecase"
	"auth-service/pkg/jwtutil"
	"x/shared/auth/middleware"
	"x/shared/utils/id"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func NewServer(cfg config.AppConfig) *http.Server {
	db, _ := config.ConnectDB()

	userRepo := repository.NewUserRepository(db)
	sf, err := id.NewSnowflake(1) // Node ID 1 for this service
	
	if err != nil {
		log.Fatalf("failed to init snowflake: %v", err)
	}

	userUC := usecase.NewUserUsecase(userRepo, sf)

	jwtGen := jwtutil.LoadAndBuild(cfg.JWT)

	auth := middleware.RequireAuth()
	authHandler := handler.NewAuthHandler(userUC, jwtGen, auth)


	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://127.0.0.1:5500", "http://localhost:5500"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r = router.SetupRoutes(r, authHandler, auth).(*chi.Mux)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}
}
