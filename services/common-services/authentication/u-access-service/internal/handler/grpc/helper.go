package hgrpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)


// ---------------------- HELPERS ----------------------

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

func int64PtrToProto(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func stringPtrToProto(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func int64OrDefault(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func stringOrDefault(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func boolOrDefault(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}