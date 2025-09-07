package service
import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)
func randomCode(digits int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil) // 10^digits
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err) // handle appropriately in prod
	}
	return fmt.Sprintf("%0*d", digits, n.Int64())
}

func formatPurpose(purpose string) string {
    p := strings.ReplaceAll(purpose, "_", " ")
    return cases.Title(language.English).String(p)
}