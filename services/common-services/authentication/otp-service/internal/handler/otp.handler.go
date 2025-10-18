package handler

import (
	"context"
	"log"

	pb "x/shared/genproto/otppb"
	"otp-service/internal/service"
)

type OTPHandler struct {
	pb.UnimplementedOTPServiceServer
	service *service.OTPService
}

func NewOTPHandler(s *service.OTPService) *OTPHandler {
	return &OTPHandler{service: s}
}

func (h *OTPHandler) GenerateOTP(ctx context.Context, req *pb.GenerateOTPRequest) (*pb.GenerateOTPResponse, error) {
	if req.UserId == "" || req.Recipient == "" {
		return &pb.GenerateOTPResponse{Ok: false, Error: "invalid request"}, nil
	}
	err := h.service.GenerateOTP(ctx, req.UserId, req.Purpose, req.Channel, req.Recipient)
	if err != nil {
		log.Printf("GenerateOTP error: %v", err)
		return &pb.GenerateOTPResponse{Ok: false, Error: err.Error()}, nil
	}
	return &pb.GenerateOTPResponse{Ok: true}, nil
}

func (h *OTPHandler) VerifyOTP(ctx context.Context, req *pb.VerifyOTPRequest) (*pb.VerifyOTPResponse, error) {
	ok, err := h.service.VerifyOTP(ctx, req.UserId, req.Purpose, req.Code)
	if err != nil {
		log.Printf("VerifyOTP error: %v", err)
		return &pb.VerifyOTPResponse{Valid: false, Error: err.Error()}, nil
	}
	return &pb.VerifyOTPResponse{Valid: ok}, nil
}
