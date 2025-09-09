package hgrpc

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func ptrToTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t != nil {
		return timestamppb.New(*t)
	}
	return nil
}
