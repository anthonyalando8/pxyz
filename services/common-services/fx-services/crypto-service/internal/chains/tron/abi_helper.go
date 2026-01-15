// internal/chains/tron/abi_helper.go
package tron

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
)

// TRC20 function signatures
const (
	TransferMethodID   = "a9059cbb" // transfer(address,uint256)
	BalanceOfMethodID  = "70a08231" // balanceOf(address)
	ApproveMethodID    = "095ea7b3" // approve(address,uint256)
	AllowanceMethodID  = "dd62ed3e" // allowance(address,address)
	TotalSupplyMethodID = "18160ddd" // totalSupply()
	NameMethodID       = "06fdde03" // name()
	SymbolMethodID     = "95d89b41" // symbol()
	DecimalsMethodID   = "313ce567" // decimals()
)

// encodeTransferData encodes transfer function call
func encodeTransferData(to string, amount *big.Int) (string, error) {
	toAddr := address.Address(to)
	
	// Encode to address (32 bytes)
	toParam := common.LeftPadBytes(toAddr.Bytes(), 32)
	
	// Encode amount (32 bytes)
	amountParam := common.LeftPadBytes(amount.Bytes(), 32)
	
	// Combine
	data := TransferMethodID + hex.EncodeToString(toParam) + hex.EncodeToString(amountParam)
	
	return data, nil
}

// encodeBalanceOfData encodes balanceOf function call
func encodeBalanceOfData(owner string) (string, error) {
	ownerAddr := address.Address(owner)
	
	// Encode owner address (32 bytes)
	ownerParam := common.LeftPadBytes(ownerAddr.Bytes(), 32)
	
	data := BalanceOfMethodID + hex.EncodeToString(ownerParam)
	
	return data, nil
}

// decodeUint256 decodes uint256 from bytes
func decodeUint256(data []byte) *big.Int {
	return new(big.Int).SetBytes(data)
}

// decodeAddress decodes address from bytes
func decodeAddress(data []byte) (string, error) {
	if len(data) < 32 {
		return "", fmt.Errorf("invalid data length for address")
	}
	
	// Address is in last 20 bytes of 32-byte word
	addrBytes := data[12:32]
	addr := address.Address(addrBytes)
	
	return addr.String(), nil
}

// decodeTransferEvent decodes Transfer event
// Transfer(address indexed from, address indexed to, uint256 value)
func decodeTransferEvent(topics [][]byte, data []byte) (from, to string, amount *big. Int, err error) {
	if len(topics) < 3 {
		return "", "", nil, fmt.Errorf("invalid topics length")
	}
	
	// Topic 0 is event signature
	// Topic 1 is from address
	// Topic 2 is to address
	// Data is amount
	
	fromAddr, err := decodeAddress(topics[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to decode from address: %w", err)
	}
	
	toAddr, err := decodeAddress(topics[2])
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to decode to address: %w", err)
	}
	
	value := decodeUint256(data)
	
	return fromAddr, toAddr, value, nil
}