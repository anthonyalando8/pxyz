// internal/chains/tron/utils.go (NEW FILE)

package tron

import (
	"fmt"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

// parseAddress safely converts a Base58 TRON address to Address type
func parseAddress(addr string) (address.Address, error) {
	parsed, err := address.Base58ToAddress(addr)
	if err != nil {
		return address.Address{}, fmt.Errorf("invalid TRON address %s: %w", addr, err)
	}
	return parsed, nil
}

// mustParseAddress panics if address is invalid (use for constants only)
func mustParseAddress(addr string) address.Address {
	parsed, err := parseAddress(addr)
	if err != nil {
		panic(err)
	}
	return parsed
}

// validateTronAddress validates TRON address format
func validateTronAddress(addr string) error {
	_, err := address.Base58ToAddress(addr)
	return err
}