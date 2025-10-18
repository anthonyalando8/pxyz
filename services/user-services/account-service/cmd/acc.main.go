package main

import (
	"account-service/internal/server"
	"log"
	"net"

	pb "x/shared/genproto/accountpb"

	"google.golang.org/grpc"
)

func main() {
	srv := server.NewServer()
	defer srv.DB.Close()
	if srv.Rdb != nil {
		defer srv.Rdb.Close()
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAccountServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", ":8004")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("gRPC server running on :8004")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
