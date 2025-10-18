package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"otp-service/internal/config"
	"otp-service/internal/repository"
	"otp-service/internal/rate"
	"otp-service/internal/service"
	"otp-service/internal/handler"
	pb "x/shared/genproto/otppb"
	"x/shared/email"
	smsclient "x/shared/sms"
	notificationclient "x/shared/notification" // ✅ added


	"github.com/jackc/pgx/v5/pgxpool"
		"x/shared/utils/cache"

	"x/shared/utils/id"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()

	// pgx pool
	dbpool, err := pgxpool.New(context.Background(), cfg.DBConnString)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer dbpool.Close()

	// redis
	cache := cache.NewCache([]string{cfg.RedisAddr}, cfg.RedisPass, false)


	notificationCli := notificationclient.NewNotificationService() // ✅ create notification client


	// snowflake
	sf, err := id.NewSnowflake(5)
	if err != nil { log.Fatalf("sf: %v", err) }

	// repos & limiter & service
	otpRepo := repository.NewOTPRepo(dbpool)
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	lim := rate.NewLimiter(cache, cfg.OTP_Window, cfg.OTP_MaxPerWindow, cfg.OTP_Cooldown)
	otpSvc:= service.NewOTPService(otpRepo, lim, sf, emailCli, smsCli,cache, cfg.OTP_TTL, notificationCli)
	// grpc server
	grpcServer := grpc.NewServer()
	otpHandler := handler.NewOTPHandler(otpSvc)
	pb.RegisterOTPServiceServer(grpcServer, otpHandler)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil { log.Fatalf("listen: %v", err) }

	go func() {
		log.Printf("OTP gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	// graceful stop
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
	grpcServer.GracefulStop()
}
