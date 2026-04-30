package core

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/neerajhoney2-code/billionaire/crypto"
)

// TxType classifies what a transaction does.
type TxType string

const (
	TxTransfer    TxType = "transfer"
	TxStake       TxType = "stake"
	TxUnstake     TxType = "unstake"
	TxRegisterMed TxType = "register_medicine"
	TxTransferMed TxType = "transfer_medicine"
	TxDispenseMed TxType = "dispense_medicine"
	TxRecallMed   TxType = "recall_medicine"
	TxDeployCtx   TxType = "deploy_contract"
	TxCallCtx     TxType = "call_contract"
	TxCoinbase    TxType = "coinbase"
)

// Transaction is the atomic unit recorded on-chain.
type Transaction struct {
	ID        string `json:"id"`
	Type      TxType `json:"type"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Amount    uint64 `json:"amount"`
	Fee       uint64 `json:"fee"`
	Nonce     uint64 `json:"nonce"`
	Data      []byte `json:"data,omitempty"` // pharma payload / contract bytecode
	PublicKey string `json:"public_key"`     // sender's compressed public key (hex)
	Signature string `json:"signature"`      // hex DER ECDSA over signing payload
	Timestamp int64  `json:"timestamp"`
}

// signingPayload is the deterministic string that is signed.
func (tx *Transaction) signingPayload() []byte {
	payload := fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d|%x",
		tx.Type, tx.Sender, tx.Recipient, tx.ID, tx.Amount, tx.Fee, tx.Nonce, tx.Data)
	return []byte(payload)
}

// ComputeID sets tx.ID as the hash of its fields (call before signing).
func (tx *Transaction) ComputeID() {
	raw := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d|%x",
		tx.Type, tx.Sender, tx.Recipient, tx.Amount, tx.Fee, tx.Nonce, tx.Timestamp, tx.Data)
	tx.ID = crypto.HashBytes([]byte(raw))
}

// Verify checks the signature. Coinbase transactions are always valid.
func (tx *Transaction) Verify() bool {
	if tx.Type == TxCoinbase {
		return true
	}
	if tx.PublicKey == "" || tx.Signature == "" {
		return false
	}
	return crypto.Verify(tx.PublicKey, tx.signingPayload(), tx.Signature)
}

// NewTransfer builds an unsigned value transfer transaction.
func NewTransfer(sender, recipient string, amount, fee, nonce uint64, pubKey string) *Transaction {
	tx := &Transaction{
		Type:      TxTransfer,
		Sender:    sender,
		Recipient: recipient,
		Amount:    amount,
		Fee:       fee,
		Nonce:     nonce,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}
	tx.ComputeID()
	return tx
}

// NewStakeTx builds an unsigned stake transaction (amount = tokens to stake).
func NewStakeTx(sender string, amount, fee, nonce uint64, pubKey string) *Transaction {
	tx := &Transaction{
		Type:      TxStake,
		Sender:    sender,
		Recipient: sender, // staking is self-directed
		Amount:    amount,
		Fee:       fee,
		Nonce:     nonce,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}
	tx.ComputeID()
	return tx
}

// NewUnstakeTx builds an unsigned unstake transaction.
func NewUnstakeTx(sender string, amount, fee, nonce uint64, pubKey string) *Transaction {
	tx := &Transaction{
		Type:      TxUnstake,
		Sender:    sender,
		Recipient: sender,
		Amount:    amount,
		Fee:       fee,
		Nonce:     nonce,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}
	tx.ComputeID()
	return tx
}

// NewPharmaTx builds an unsigned pharma operation transaction.
func NewPharmaTx(txType TxType, sender string, data interface{}, fee, nonce uint64, pubKey string) (*Transaction, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	tx := &Transaction{
		Type:      txType,
		Sender:    sender,
		Recipient: "pharma_registry",
		Amount:    0,
		Fee:       fee,
		Nonce:     nonce,
		Data:      raw,
		PublicKey: pubKey,
		Timestamp: time.Now().UnixNano(),
	}
	tx.ComputeID()
	return tx, nil
}

// NewCoinbase creates a genesis allocation transaction.
func NewCoinbase(recipient string, amount uint64) *Transaction {
	tx := &Transaction{
		Type:      TxCoinbase,
		Sender:    "genesis",
		Recipient: recipient,
		Amount:    amount,
		Timestamp: time.Now().UnixNano(),
	}
	tx.ComputeID()
	return tx
}
