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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPass,
	})
	defer rdb.Close()

	// snowflake
	sf, err := id.NewSnowflake(5)
	if err != nil { log.Fatalf("sf: %v", err) }

	// repos & limiter & service
	otpRepo := repository.NewOTPRepo(dbpool)
	emailCli := emailclient.NewEmailClient()
	smsCli := smsclient.NewSMSClient()
	lim := rate.NewLimiter(rdb, cfg.OTP_Window, cfg.OTP_MaxPerWindow, cfg.OTP_Cooldown)
	otpSvc:= service.NewOTPService(otpRepo, lim, sf, emailCli, smsCli,rdb, cfg.OTP_TTL)
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
