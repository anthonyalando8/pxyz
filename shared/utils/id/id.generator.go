package id

import (
	"fmt"
	"strconv"
	"sync"
	"time"
	"crypto/rand"
	"github.com/oklog/ulid/v2"

	"encoding/hex"
	"math/big"

	"strings"
	"sync/atomic"	
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

	now := time.Now().UnixNano() / 1e6 // ms

	// Handle clock rollback
	if now < s.timestamp {
		// Wait until clock catches up
		for now < s.timestamp {
			now = time.Now().UnixNano() / 1e6
		}
	}

	if now == s.timestamp {
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			// sequence overflow → wait for next ms
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
// ID format: <prefix><random+timestamp> total length ~12 chars.
func GenerateID(prefix string) string {
	// 1. Timestamp in milliseconds (6 hex chars should be enough for uniqueness in high frequency)
	ts := time.Now().UnixNano() / 1e6 // milliseconds
	tsHex := fmt.Sprintf("%06x", ts&0xFFFFFF) // take last 3 bytes to fit into 6 hex chars

	// 2. Random 3 bytes (6 hex chars)
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	randHex := hex.EncodeToString(b)

	// 3. Combine prefix + timestamp + random
	id := fmt.Sprintf("%s%s%s", prefix, tsHex, randHex)

	return id
}

var globalCounter uint64

// GenerateWalletID creates unique, obfuscated, uppercase alphanumeric IDs.
// Example: DN4F7X2K
func GenerateWalletID(prefix string, length int) string {
	if length <= len(prefix) {
		panic("length must be greater than prefix length")
	}

	// Monotonic counter + timestamp (ensures ordering + uniqueness)
	counter := atomic.AddUint64(&globalCounter, 1)
	ts := time.Now().UnixNano() / 1e6 // milliseconds

	// Base number = timestamp + counter
	num := big.NewInt(ts + int64(counter))
	base36 := strings.ToUpper(num.Text(36))

	// Add a random 2-char salt to break predictability
	salt := randomBase36(2)

	// Combine
	idBody := base36 + salt

	// Ensure fixed length
	needed := length - len(prefix)
	if len(idBody) > needed {
		idBody = idBody[len(idBody)-needed:]
	} else if len(idBody) < needed {
		idBody = strings.Repeat("0", needed-len(idBody)) + idBody
	}

	return prefix + idBody
}

func randomBase36(n int) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		out[i] = chars[num.Int64()]
	}
	return string(out)
}

func GenerateTransactionID(prefix string) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	
	// Use milliseconds mod 10000 (last 4 digits)
	timestamp := time.Now().UnixMilli() % 10000
	
	// Generate 4 random chars
	b := make([]byte, 4)
	for i := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[num.Int64()]
	}
	
	return fmt.Sprintf("%s-%04d%s", prefix, timestamp, string(b))
}
// password usage example

// // For partner admin accounts (default)
// password, _ := id.GeneratePassword()
// // Output: BrightTiger42!, QuickEagle87@, BoldDragon23#

// // For high-security accounts
// password, _ := id.GenerateStrongPassword()
// // Output: SwiftPhoenix7294!@x, BraveWolf4567#$p

// // For simple cases
// password, _ := id.GenerateSimplePassword()
// // Output: tiger42!, eagle87@

// // For passphrases
// passphrase, _ := id.GeneratePassphrase()
// // Output: Quick-Bright-Tiger-42, Swift-Bold-Eagle-87

// // With custom policy
// policy := id.PasswordPolicy{
// 	MinLength:      12,
// 	RequireUpper:   true,
// 	RequireLower:   true,
// 	RequireDigit:   true,
// 	RequireSpecial: true,
// 	UseWords:       true,
// }
// password, _ := id.GeneratePasswordWithPolicy(policy)

// // Validate password strength
// strong, score, feedback := id.ValidatePasswordStrength("BrightTiger42!")
// // strong: true, score: 7, feedback: []