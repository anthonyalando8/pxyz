package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"auth-service/internal/domain"
	sessionpb "x/shared/genproto/sessionpb"
)

func (h *AuthHandler) createSessionHelper(
	ctx context.Context,
	userID string,
	deviceID *string,
	devMetadata *any,
	geoLocation *string,
	r *http.Request,
) (*domain.Session, error) {

	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	// Determine device ID (use provided or generate new)
	var device string
	if deviceID == nil {
		device = h.uc.Sf.Generate()
	} else {
		device = *deviceID
	}

	// Capture contextual info
	ip := extractClientIP(r)
	ua := r.Header.Get("User-Agent")
	now := time.Now()

	// Generate JWT
	token, _, err := h.jwtGen.Generate(userID, device)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Prepare device metadata as JSON (if provided)
	var devMetaStr string
	if devMetadata != nil {
		if b, err := json.Marshal(devMetadata); err == nil {
			devMetaStr = string(b)
		} else {
			return nil, fmt.Errorf("failed to marshal device metadata: %w", err)
		}
	}

	// Call gRPC CreateSession
	grpcResp, err := h.auth.Client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
		UserId:         userID,
		AuthToken:      token,
		DeviceId:       device,
		IpAddress:      ip,
		UserAgent:      ua,
		GeoLocation:    getStringOrDefault(geoLocation),
		DeviceMetadata: devMetaStr,
		IsActive:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session via gRPC: %w", err)
	}

	// Build domain session object from gRPC response
	return &domain.Session{
		ID:           grpcResp.Session.Id,
		UserID:       userID,
		AuthToken:    token,
		DeviceID:     &device,
		IPAddress:    &ip,
		UserAgent:    &ua,
		GeoLocation:  geoLocation,
		DeviceMeta:   devMetadata,
		IsActive:     true,
		LastSeenAt:   &now,
		CreatedAt:    now,
	}, nil
}

func getStringOrDefault(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}


func extractClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	return ip
}
