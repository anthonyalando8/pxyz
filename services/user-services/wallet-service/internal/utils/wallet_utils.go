package utils

import (
	"time"
	"crypto/rand"
	"github.com/oklog/ulid/v2"
)

func GenerateUUID(prefix string) string {
    id := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(rand.Reader, 0))
    return prefix + "_" + id.String()
}
