package handler

import (
	"context"
	"net"
	"net/http"

	"auth-service/internal/domain"
	"x/shared/genproto/authpb"
)

func (h *AuthHandler) createSessionHelper(ctx context.Context, userID string, r *http.Request) (*domain.Session, error) {
	device := h.uc.Sf.Generate() // Generate a unique device ID
	ip := extractClientIP(r)

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
