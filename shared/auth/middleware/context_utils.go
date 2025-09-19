package middleware

import (
	"context"
	"net/http"
	"x/shared/auth/pkg/jwtutil"
	authpb "x/shared/genproto/sessionpb"
)

type contextKey string

const (
	ContextUserID contextKey = "userID"
	ContextToken  contextKey = "token"
	ContextSessionType contextKey = "sessionType"
	ContextExtraData contextKey = "extraData"
	ContextSessionPurpose contextKey = "purpose"
	ContextDeviceID contextKey = "deviceID"
	ContextRole contextKey = "role"
	ContextUserType contextKey = "userType"
)

func GetUserID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(ContextUserID).(string)
	return val, ok
}

func GetToken(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(ContextToken).(string)
	return val, ok
}

func setContextValues(r *http.Request, claims *jwtutil.Claims, token string, resp *authpb.ValidateSessionResponse) *http.Request {
	ctx := context.WithValue(r.Context(), ContextUserID, claims.UserID)
	ctx = context.WithValue(ctx, ContextToken, token)
	ctx = context.WithValue(ctx, ContextDeviceID, claims.Device)
	ctx = context.WithValue(ctx, ContextRole, claims.Role)
	ctx = context.WithValue(ctx, ContextUserType, claims.UserType)

	if resp != nil {
		ctx = context.WithValue(ctx, ContextSessionType, resp.SessionType)
		ctx = context.WithValue(ctx, ContextSessionPurpose, resp.Purpose)
	}

	if len(claims.ExtraData) > 0 {
		ctx = context.WithValue(ctx, ContextExtraData, claims.ExtraData)
	}

	return r.WithContext(ctx)
}
