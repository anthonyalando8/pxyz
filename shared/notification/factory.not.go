package notificationclient

import (
	"log"
	"os"

	notificationpb "x/shared/genproto/shared/notificationpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationService struct {
	Client notificationpb.NotificationServiceClient
	Conn   *grpc.ClientConn
}

func NewNotificationService() *NotificationService {
	addr := getEnv("NOTIFICATION_SERVICE_ADDR", "notification-service:8014")

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Notification service at %s: %v", addr, err)
	}

	client := notificationpb.NewNotificationServiceClient(conn)
	return &NotificationService{
		Client: client,
		Conn:   conn,
	}
}

func (n *NotificationService) Close() error {
	return n.Conn.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
