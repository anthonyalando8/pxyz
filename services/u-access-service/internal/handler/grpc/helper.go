package hgrpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)


// ---------------------- HELPERS ----------------------

// Convert *int64 to int64 (returns 0 if nil)
func int64PtrToProto(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func ptrToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t != nil {
		return timestamppb.New(*t)
	}
	return nil
}

func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrToInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func Int64Ptr(v int64) *int64 {
    return &v
}