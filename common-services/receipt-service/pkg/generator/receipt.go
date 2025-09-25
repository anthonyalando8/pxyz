package generator

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/oklog/ulid/v2"
)

const (
	prefix    = "TR"
	codeLen   = 10             // length of random part (excluding prefix)
	maxRetries = 5
	base62Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Generator provides methods to generate unique receipt codes.
type Generator struct{}

// NewGenerator returns a new receipt code generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates a new receipt code (ULID-based) with prefix.
// Returns a 12-char code like TR01F7K8J3M
func (g *Generator) Generate() string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(t), entropy)

	// Take first `codeLen` characters from ULID
	return prefix + id.String()[0:codeLen]
}

// GenerateRandom generates a fully random Base62 code prefixed with TR
func (g *Generator) GenerateRandom() string {
	result := make([]byte, codeLen)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(base62Chars))))
		result[i] = base62Chars[n.Int64()]
	}
	return prefix + string(result)
}

// GenerateUnique attempts to generate a code and ensures uniqueness
// by checking with the provided callback function.
// If checkFunc is nil, no uniqueness check is performed here.
func (g *Generator) GenerateUnique(checkFunc func(string) bool) (string, error) {
    for i := 0; i < maxRetries; i++ {
        code := g.Generate()
        if checkFunc == nil || !checkFunc(code) {
            return code, nil
        }
    }
    // fallback to random Base62 after retries
    for i := 0; i < maxRetries; i++ {
        code := g.GenerateRandom()
        if checkFunc == nil || !checkFunc(code) {
            return code, nil
        }
    }
    return "", fmt.Errorf("failed to generate unique receipt code after %d attempts", maxRetries*2)
}
