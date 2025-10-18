package accservice

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func GenerateSysUsername(firstName, lastName string) string {
	rand.Seed(time.Now().UnixNano())

	// base: first letter of firstName + lastName
	base := ""
	if firstName != "" {
		base += strings.ToLower(string(firstName[0]))
	}
	if lastName != "" {
		base += strings.ToLower(lastName)
	}

	// fallback if both empty
	if base == "" {
		base = "user"
	}

	// add random 4-digit number
	return fmt.Sprintf("%s%d", base, rand.Intn(9000)+1000)
}
