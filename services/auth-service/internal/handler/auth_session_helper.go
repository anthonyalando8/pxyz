package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"auth-service/internal/domain"
	"x/shared/genproto/authpb"
)

func (h *AuthHandler) createSessionHelper(
	ctx context.Context,
	userID string,
	deviceID *string,
	devMetadata *any,
	geoLocation *string,
	r *http.Request)(*domain.Session, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	var device string
	if deviceID == nil {
		device = h.uc.Sf.Generate() // Generate a unique device ID
	}else{
		device = *deviceID
	}

	// 3. Capture contextual info
	ip := extractClientIP(r)
    ua := r.Header.Get("User-Agent")
    now := time.Now()

	token, _, err := h.jwtGen.Generate(userID, device)
	if err != nil {
		return nil, err
	}

	grpcResp, err := h.auth.Client.CreateSession(ctx, &authpb.CreateSessionRequest{
		UserId: userID,
		Token:  token,
		Device: device,
		Ip:     ip,
	})
	if err != nil {
		return nil, err
	}

	return &domain.Session{
		ID:     grpcResp.SessionId,
		UserID: userID,
		AuthToken:  token,
		DeviceID: toPtr(device),
		IPAddress:     toPtr(ip),
		UserAgent:   &ua,
        GeoLocation: geoLocation,
        DeviceMeta:  devMetadata,
        IsActive:    true,
        LastSeenAt:  &now,
        CreatedAt:   now,
	}, nil
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
