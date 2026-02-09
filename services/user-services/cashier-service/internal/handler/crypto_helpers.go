package handler

import (
	"context"
	"fmt"
	"strings"
	cryptopb "x/shared/genproto/shared/accounting/cryptopb"
)

// mapCurrencyToCrypto maps currency code to blockchain chain and asset
func (h *PaymentHandler) mapCurrencyToCrypto(currency string) (string, string, error) {
	mappings := map[string]struct {
		Chain string
		Asset string
	}{
		"BTC":  {"BITCOIN", "BTC"},
		"USDT": {"TRON", "USDT"}, // USDT on TRON by default
		"TRX":  {"TRON", "TRX"},
		"ETH":  {"ETHEREUM", "ETH"},
		"USDC": {"ETHEREUM", "USDC"},
		// Add more as needed
	}

	mapping, ok := mappings[strings.ToUpper(currency)]
	if !ok {
		return "", "", fmt.Errorf("unsupported crypto currency: %s", currency)
	}

	return mapping.Chain, mapping.Asset, nil
}

// mapChainToProto maps chain string to protobuf enum
func mapChainToProto(chain string) cryptopb.Chain {
	chainMap := map[string]cryptopb.Chain{
		"BITCOIN":  cryptopb.Chain_CHAIN_BITCOIN,
		"TRON":     cryptopb.Chain_CHAIN_TRON,
		"ETHEREUM": cryptopb.Chain_CHAIN_ETHEREUM,
	}

	if protoChain, ok := chainMap[strings.ToUpper(chain)]; ok {
		return protoChain
	}

	return cryptopb.Chain_CHAIN_UNSPECIFIED
}

func mapProtoToChain(protoChain cryptopb.Chain) string {
	chainMap := map[cryptopb.Chain]string{
		cryptopb.Chain_CHAIN_BITCOIN:  "BITCOIN",
		cryptopb.Chain_CHAIN_TRON:     "TRON",
		cryptopb.Chain_CHAIN_ETHEREUM: "ETHEREUM",
	}
	if chain, ok := chainMap[protoChain]; ok {
		return chain
	}
	return "UNSPECIFIED"
}

// validateCryptoAddress validates the crypto address format
func (h *PaymentHandler) validateCryptoAddress(ctx context.Context, address, chain string) error {
	// TODO: Call crypto service to validate address format
	// For now, basic validation

	switch chain {
	case "BITCOIN":
		// Bitcoin addresses: legacy (1...), P2SH (3...), bech32 (bc1...)
		if !strings.HasPrefix(address, "1") &&
			!strings.HasPrefix(address, "3") &&
			!strings.HasPrefix(address, "bc1") &&
			!strings.HasPrefix(address, "tb1") && // testnet
			!strings.HasPrefix(address, "m") && // testnet legacy
			!strings.HasPrefix(address, "n") { // testnet P2PKH
			return fmt.Errorf("invalid Bitcoin address format")
		}

	case "TRON":
		// TRON addresses start with T (mainnet) or T (testnet)
		if (!strings.HasPrefix(address, "T") || len(address) != 34) {
			return fmt.Errorf("invalid TRON address format (must start with T and be 34 characters)")
		}

	case "ETHEREUM":
		// Ethereum addresses start with 0x and are 42 chars
		if !strings.HasPrefix(address, "0x") || len(address) != 42 {
			return fmt.Errorf("invalid Ethereum address format")
		}
	}

	return nil
}