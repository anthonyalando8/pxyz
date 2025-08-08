package middleware

import "context"

func GetUserID(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(ContextUserID).(string)
	return val, ok
}

func GetToken(ctx context.Context) (string, bool) {
	val, ok := ctx.Value(ContextToken).(string)
	return val, ok
}
