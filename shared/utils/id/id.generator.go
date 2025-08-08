package id

import (
	"fmt"
	"strconv"
	"sync"
	"time"
	"crypto/rand"
	"github.com/oklog/ulid/v2"
)

const (
	epoch           int64 = 1672531200000 // Custom epoch: 2023-01-01 UTC in ms
	nodeBits        uint8 = 10            // Supports up to 1024 nodes
	sequenceBits    uint8 = 12            // Supports up to 4096 IDs per ms per node
	nodeMax               = -1 ^ (-1 << nodeBits)
	sequenceMask          = -1 ^ (-1 << sequenceBits)
	nodeShift       uint8 = sequenceBits
	timestampShift  uint8 = sequenceBits + nodeBits
)

type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	nodeID    int64
	sequence  int64
}

func NewSnowflake(nodeID int64) (*Snowflake, error) {
	if nodeID < 0 || nodeID > int64(nodeMax) {
		return nil, ErrInvalidNode
	}
	return &Snowflake{
		nodeID: nodeID,
	}, nil
}

var ErrInvalidNode = fmt.Errorf("node ID must be between 0 and %d", nodeMax)

func (s *Snowflake) Generate() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixNano() / 1e6 // milliseconds

	if now == s.timestamp {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			// sequence overflow in same ms, wait for next ms
			for now <= s.timestamp {
				now = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		s.sequence = 0
	}

	s.timestamp = now

	id := ((now - epoch) << timestampShift) |
		(s.nodeID << nodeShift) |
		(s.sequence)

	return strconv.FormatInt(id, 10)
}

func GenerateUUID(prefix string) string {
    id := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(rand.Reader, 0))
    return prefix + "_" + id.String()
}