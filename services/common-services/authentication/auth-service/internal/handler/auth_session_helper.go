package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
    "log"

	"auth-service/internal/domain"
	"x/shared/auth/middleware"
	sessionpb "x/shared/genproto/sessionpb"
)

func (h *AuthHandler) createSessionHelper(
    ctx context.Context,
    userID string,
    isTemp bool,
    isSingleUse bool,
    purpose string,
    extraData map[string]interface{},
    deviceID *string,
    devMetadata *any,
    geoLocation *string,
    r *http.Request,
) (*domain.Session, error) {

    if userID == "" {
        return nil, errors.New("user ID is required")
    }

    // Device ID handling
    var device string
    if deviceID == nil || *deviceID == "" {
        device = h.uc.Sf.Generate()
    } else {
        device = *deviceID
    }

    // Ensure extraData is not nil
    if extraData == nil {
        extraData = make(map[string]interface{})
    }

    // Context info
    ip := extractClientIP(r)
    ua := r.Header.Get("User-Agent")

    // Marshal device metadata
    var devMetaStr string
    if devMetadata != nil {
        if b, err := json.Marshal(*devMetadata); err == nil {
            devMetaStr = string(b)
        } else {
            return nil, fmt.Errorf("failed to marshal device metadata: %w", err)
        }
    }
    extraStr := make(map[string]string)
    for k, v := range extraData {
        if strVal, ok := v.(string); ok {
            extraStr[k] = strVal
        } else {
            // optional: handle non-string values
            extraStr[k] = fmt.Sprintf("%v", v)
        }
    }


    // gRPC call
    grpcResp, err := h.auth.Client.CreateSession(ctx, &sessionpb.CreateSessionRequest{
        UserId:         userID,
        DeviceId:       device,
        IpAddress:      ip,
        UserAgent:      ua,
        GeoLocation:    getStringOrDefault(geoLocation),
        DeviceMetadata: devMetaStr,
        IsActive:       true,
        IsTemp:         isTemp,
        IsSingleUse:    isSingleUse,
        Purpose:        purpose,
        ExtraData:      extraStr,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create session via gRPC: %w", err)
    }

    now := time.Now()

    return &domain.Session{
        ID:          grpcResp.Session.Id,
        UserID:      userID,
        AuthToken:   grpcResp.Session.AuthToken,
        DeviceID:    &device,
        IPAddress:   &ip,
        UserAgent:   &ua,
        GeoLocation: geoLocation,
        DeviceMeta:  devMetadata,
        IsActive:    grpcResp.Session.IsActive,
        IsTemp:      grpcResp.Session.IsTemp,
        IsSingleUse: grpcResp.Session.IsSingleUse,
        IsUsed:      grpcResp.Session.IsUsed,
        LastSeenAt:  &now,
        CreatedAt:   now,
    }, nil
}

func (h *AuthHandler) logoutSessionBg(ctx context.Context){
    currentTokenVal := ctx.Value(middleware.ContextToken)
    currentToken, _ := currentTokenVal.(string)

    if currentToken != "" {
        go func(token string) {
            delResp, delErr := h.sessionClient.DeleteSession(context.Background(), &sessionpb.DeleteSessionRequest{
                Token: token,
            })
            if delErr != nil {
                log.Printf("[LogoutSession] Failed to delete old session: %v", delErr)
            } else {
                log.Printf("[LogoutSession] Old session deleted successfully: %+v", delResp)
            }
        }(currentToken)
    }
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
