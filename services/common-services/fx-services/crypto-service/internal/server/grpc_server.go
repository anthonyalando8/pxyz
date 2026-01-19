// internal/server/grpc_server.go
package server

import (
	"crypto-service/internal/handler"
	"fmt"
	"net"

	pb "x/shared/genproto/shared/accounting/cryptopb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	server             *grpc.Server
	walletHandler      *handler.WalletHandler
	transactionHandler *handler.TransactionHandler
	depositHandler     *handler.DepositHandler
	cryptoHandler      *handler. CryptoHandler
	logger             *zap.Logger
	port               int
}

func NewGRPCServer(
	walletHandler *handler.WalletHandler,
	transactionHandler *handler.TransactionHandler,
	depositHandler *handler.DepositHandler,
	cryptoHandler *handler.CryptoHandler,
	logger *zap.Logger,
	port int,
) *GRPCServer {
	return &GRPCServer{
		walletHandler:      walletHandler,
		transactionHandler: transactionHandler,
		depositHandler:     depositHandler,
		cryptoHandler:      cryptoHandler,
		logger:             logger,
		port:               port,
	}
}

// Start starts the gRPC server
func (s *GRPCServer) Start() error {
	// Create listener
	lis, err := net.Listen("tcp", fmt. Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	
	// Create gRPC server with options
	s.server = grpc.NewServer(
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
		// Add interceptors for logging, auth, etc.
	)
	
	// Register services
	pb.RegisterWalletServiceServer(s.server, s.walletHandler)
	pb.RegisterTransactionServiceServer(s.server, s.transactionHandler)
	pb.RegisterDepositServiceServer(s.server, s.depositHandler)
	pb.RegisterCryptoServiceServer(s.server, s.cryptoHandler)
	
	// Register reflection service (for grpcurl, Postman, etc.)
	reflection.Register(s.server)
	
	s.logger.Info("Starting gRPC server",
		zap.Int("port", s.port),
	)
	
	// Start serving
	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	
	return nil
}

// Stop gracefully stops the gRPC server
func (s *GRPCServer) Stop() {
	s.logger.Info("Stopping gRPC server")
	s.server.GracefulStop()
}