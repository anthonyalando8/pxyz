package main

import (
	"account-service/internal/server"
	"log"
	"net"

	"google.golang.org/grpc"
	pb "x/shared/genproto/accountpb"
)

func main() {
    srv := server.NewServer()
    defer srv.DB.Close()
    if srv.Rdb != nil {
        defer srv.Rdb.Close()
    }

    grpcServer := grpc.NewServer()
    pb.RegisterAccountServiceServer(grpcServer, srv)

    lis, err := net.Listen("tcp", ":50056")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    log.Println("gRPC server running on :50056")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}

