// handler/grpc_partner_api.go
package handler

import (
	"context"
	"log"
	partnersvcpb "x/shared/genproto/partner/svcpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GenerateAPICredentials creates new API credentials for a partner
func (h *GRPCPartnerHandler) GenerateAPICredentials(
	ctx context.Context,
	req *partnersvcpb.GenerateAPICredentialsRequest,
) (*partnersvcpb.APICredentialsResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	apiKey, apiSecret, err := h.uc.GenerateAPICredentials(ctx, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] GenerateAPICredentials failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to generate API credentials: %v", err)
	}

	return &partnersvcpb.APICredentialsResponse{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
		PartnerId: req.PartnerId,
	}, nil
}

// RevokeAPICredentials removes API access
func (h *GRPCPartnerHandler) RevokeAPICredentials(
	ctx context.Context,
	req *partnersvcpb.RevokeAPICredentialsRequest,
) (*partnersvcpb.RevokeAPICredentialsResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	if err := h.uc.RevokeAPICredentials(ctx, req.PartnerId); err != nil {
		log.Printf("[ERROR] RevokeAPICredentials failed: %v", err)
		return &partnersvcpb.RevokeAPICredentialsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &partnersvcpb.RevokeAPICredentialsResponse{
		Success: true,
		Message: "API credentials revoked successfully",
	}, nil
}

// RotateAPISecret generates new secret
func (h *GRPCPartnerHandler) RotateAPISecret(
	ctx context.Context,
	req *partnersvcpb.RotateAPISecretRequest,
) (*partnersvcpb.APICredentialsResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	apiSecret, err := h.uc.RotateAPISecret(ctx, req.PartnerId)
	if err != nil {
		log.Printf("[ERROR] RotateAPISecret failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to rotate API secret: %v", err)
	}

	partner, _ := h.uc.GetPartnerByID(ctx, req.PartnerId)
	apiKey := ""
	if partner != nil && partner.APIKey != nil {
		apiKey = *partner.APIKey
	}

	return &partnersvcpb.APICredentialsResponse{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
		PartnerId: req.PartnerId,
	}, nil
}

// UpdateAPISettings updates API configuration
func (h *GRPCPartnerHandler) UpdateAPISettings(
	ctx context.Context,
	req *partnersvcpb.UpdateAPISettingsRequest,
) (*partnersvcpb.PartnerResponse, error) {
	if req.PartnerId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "partner_id is required")
	}

	err := h.uc.UpdateAPISettings(ctx, req.PartnerId, req.IsApiEnabled, int(req.ApiRateLimit), req.AllowedIps)
	if err != nil {
		log.Printf("[ERROR] UpdateAPISettings failed: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to update API settings: %v", err)
	}

	partner, err := h.uc.GetPartnerByID(ctx, req.PartnerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch updated partner: %v", err)
	}

	return &partnersvcpb.PartnerResponse{
		Partner: partner.ToProto(),
	}, nil
}