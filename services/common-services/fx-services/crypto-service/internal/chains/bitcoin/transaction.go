// internal/chains/bitcoin/transaction. go
package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	//"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TransactionBuilder builds Bitcoin transactions
type TransactionBuilder struct {
	network *chaincfg.Params
	tx      *wire.MsgTx
	utxos   []UTXO
	inputs  []txInput
}

type txInput struct {
	utxo       UTXO
	privateKey *btcec.PrivateKey
	address    btcutil.Address
}

// NewTransactionBuilder creates a new transaction builder
func NewTransactionBuilder(network string) (*TransactionBuilder, error) {
	params, err := getNetworkParams(network)
	if err != nil {
		return nil, err
	}

	return &TransactionBuilder{
		network:  params,
		tx:      wire.NewMsgTx(wire.TxVersion),
		inputs:  make([]txInput, 0),
	}, nil
}

// AddInput adds a UTXO input to the transaction
func (tb *TransactionBuilder) AddInput(utxo UTXO, privateKeyWIF string) error {
	// Decode WIF private key
	wif, err := btcutil.DecodeWIF(privateKeyWIF)
	if err != nil {
		return fmt.Errorf("invalid WIF:  %w", err)
	}

	// Parse previous transaction hash
	prevHash, err := chainhash.NewHashFromStr(utxo.TxID)
	if err != nil {
		return fmt.Errorf("invalid txid: %w", err)
	}

	// Create transaction input
	txIn := wire.NewTxIn(
		wire.NewOutPoint(prevHash, utxo. Vout),
		nil,
		nil,
	)

	// Set sequence (for RBF - Replace By Fee)
	txIn.Sequence = 0xfffffffd

	tb.tx.AddTxIn(txIn)

	// Get address from UTXO (would need to be passed or derived)
	// For now, derive from public key
	publicKey := wif.PrivKey.PubKey()
	pubKeyHash := btcutil.Hash160(publicKey.SerializeCompressed())
	address, err := btcutil.NewAddressPubKeyHash(pubKeyHash, tb.network)
	if err != nil {
		return fmt. Errorf("failed to create address: %w", err)
	}

	// Store input info for signing
	tb.inputs = append(tb.inputs, txInput{
		utxo:       utxo,
		privateKey:  wif. PrivKey,
		address:     address,
	})

	return nil
}

// AddOutput adds an output to the transaction
func (tb *TransactionBuilder) AddOutput(address string, amountSats int64) error {
	// Decode address
	addr, err := btcutil.DecodeAddress(address, tb.network)
	if err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	// Create pay-to-address script
	pkScript, err := txscript. PayToAddrScript(addr)
	if err != nil {
		return fmt. Errorf("failed to create script: %w", err)
	}

	// Add transaction output
	txOut := wire.NewTxOut(amountSats, pkScript)
	tb.tx.AddTxOut(txOut)

	return nil
}

// Sign signs all inputs
func (tb *TransactionBuilder) Sign() error {
	for i, input := range tb.inputs {
		// Create pay-to-pubkey-hash script
		pkScript, err := txscript.PayToAddrScript(input.address)
		if err != nil {
			return fmt.Errorf("failed to create pkScript: %w", err)
		}

		// Create signature hash
		sigHash, err := txscript. CalcSignatureHash(
			pkScript,
			txscript. SigHashAll,
			tb.tx,
			i,
		)
		if err != nil {
			return fmt.Errorf("failed to calculate signature hash: %w", err)
		}

		// Sign
		signature := ecdsa.Sign(input.privateKey, sigHash)

		// Serialize signature with SIGHASH_ALL flag
		sigBytes := append(signature.Serialize(), byte(txscript.SigHashAll))

		// Create signature script
		publicKey := input.privateKey.PubKey().SerializeCompressed()
		sigScript, err := txscript.NewScriptBuilder().
			AddData(sigBytes).
			AddData(publicKey).
			Script()
		if err != nil {
			return fmt.Errorf("failed to build signature script: %w", err)
		}

		// Set signature script
		tb.tx.TxIn[i].SignatureScript = sigScript
	}

	return nil
}

// Serialize returns the raw transaction hex
func (tb *TransactionBuilder) Serialize() (string, error) {
	var buf bytes.Buffer
	if err := tb.tx.Serialize(&buf); err != nil {
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}

	return hex.EncodeToString(buf.Bytes()), nil
}

// GetTxHash returns the transaction hash
func (tb *TransactionBuilder) GetTxHash() string {
	return tb.tx.TxHash().String()
}

// CalculateFee calculates the transaction fee
func (tb *TransactionBuilder) CalculateFee() int64 {
	var totalInput int64
	for _, input := range tb.inputs {
		totalInput += input.utxo. Value
	}

	var totalOutput int64
	for _, output := range tb.tx.TxOut {
		totalOutput += output.Value
	}

	return totalInput - totalOutput
}

// EstimateSize estimates the transaction size in vBytes
func (tb *TransactionBuilder) EstimateSize() int {
	// Rough estimation: 
	// Base:  10 bytes
	// Input: ~148 bytes each (P2PKH)
	// Output: ~34 bytes each
	
	baseSize := 10
	inputSize := len(tb. tx.TxIn) * 148
	outputSize := len(tb. tx.TxOut) * 34

	return baseSize + inputSize + outputSize
}