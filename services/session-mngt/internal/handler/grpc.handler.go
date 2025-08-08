package handler

import (
	"context"
	"log"
	"time"

	"session-service/internal/usecase"
	pb "x/shared/genproto/authpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	uc *usecase.SessionUsecase
}

func NewAuthHandler(uc *usecase.SessionUsecase) *AuthHandler {
	return &AuthHandler{uc: uc}
}

func (h *AuthHandler) ValidateSession(ctx context.Context, req *pb.ValidateSessionRequest) (*pb.ValidateSessionResponse, error) {
	session, err := h.uc.SessionRepo.GetSessionByToken(ctx, req.Token)
	if err != nil {
		return &pb.ValidateSessionResponse{
			Valid: false,
			Error: "session not found",
		}, nil
	}

	return &pb.ValidateSessionResponse{
		Valid:  true,
		UserId: session.UserID,
	}, nil
}

func (h *AuthHandler) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	return h.uc.CreateSession(ctx, req)
}


func (h *AuthHandler) ListSessions(ctx context.Context, req *pb.ListSessionsRequest) (*pb.ListSessionsResponse, error) {
	sessions, err := h.uc.GetSessionsByUserID(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to list sessions: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list sessions")
	}

	var protoSessions []*pb.Session
	for _, session := range sessions {
		protoSessions = append(protoSessions, &pb.Session{
			Id:  session.ID,
			Device:     session.Device,
			Ip:         session.IP,
			LastActive: session.LastActive.Format(time.RFC3339),
			CreatedAt:  session.CreatedAt.Format(time.RFC3339),
		})
	}

	return &pb.ListSessionsResponse{Sessions: protoSessions}, nil
}


func (h *AuthHandler) DeleteSession(ctx context.Context, req *pb.DeleteSessionRequest) (*pb.Empty, error) {
	err := h.uc.DeleteSession(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (h *AuthHandler) DeleteAllSessions(ctx context.Context, req *pb.DeleteAllSessionsRequest) (*pb.Empty, error) {
	err := h.uc.DeleteAllSessions(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (h *AuthHandler) DeleteSessionByID(ctx context.Context, req *pb.DeleteSessionByIDRequest) (*pb.Empty, error) {
	err := h.uc.DeleteSessionByID(ctx, req.SessionId)
	if err != nil {
		log.Printf("Failed to delete session: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to delete session")
	}
	return &pb.Empty{}, nil
}