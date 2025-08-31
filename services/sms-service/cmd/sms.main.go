package main

import (
    "log"
    "net"

    "google.golang.org/grpc"
    "sms-service/internal/handler"
    "sms-service/internal/usecase"
    "x/shared/genproto/smswhatsapppb"
	"sms-service/internal/config"
)

func main() {
    // load from env/config
	cfg := config.Load()
    smsKey := cfg.SmsKey
    waKey := cfg.WaKey
    smsURL := cfg.SmsURL // "https://smsportal.hostpinnacle.co.ke/api"
    waURL := cfg.WaURL // "https://whatsappprovider.com/api"
    sender := cfg.Sender
	userId := cfg.UserId
	password := cfg.Password // replace with actual password

    uc := usecase.NewMessageUsecase(smsKey, waKey, smsURL, waURL, sender, userId, password)
    h := handler.NewMessageHandler(uc)

    lis, err := net.Listen("tcp", ":50059")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()
    smswhatsapppb.RegisterSMSWhatsAppServiceServer(grpcServer, h)

    log.Println("SMS/WhatsApp Service running on :50059")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
