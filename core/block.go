package core

import (
	"encoding/json"
	"fmt"

	"github.com/neerajhoney2-code/billionaire/crypto"
)

// Block is a single unit of the blockchain.
type Block struct {
	Index     uint64         `json:"index"`
	Timestamp int64          `json:"timestamp"`
	Txs       []*Transaction `json:"transactions"`
	Validator string         `json:"validator"`
	PrevHash  string         `json:"prev_hash"`
	Hash      string         `json:"hash"`
	Signature string         `json:"signature"` // validator signs the block hash
	Reward    uint64         `json:"reward"`
}

// hashPayload is a stable JSON-serialisable struct used for hashing.
type hashPayload struct {
	Index     uint64   `json:"index"`
	Timestamp int64    `json:"timestamp"`
	Validator string   `json:"validator"`
	PrevHash  string   `json:"prev_hash"`
	TxIDs     []string `json:"tx_ids"`
	Reward    uint64   `json:"reward"`
}

// ComputeHash calculates the canonical SHA-256 hash of the block.
func ComputeHash(index uint64, timestamp int64, txs []*Transaction, validator, prevHash string, reward uint64) string {
	ids := make([]string, len(txs))
	for i, tx := range txs {
		ids[i] = tx.ID
	}
	p := hashPayload{
		Index:     index,
		Timestamp: timestamp,
		Validator: validator,
		PrevHash:  prevHash,
		TxIDs:     ids,
		Reward:    reward,
	}
	raw, _ := json.Marshal(p)
	return crypto.HashBytes(raw)
}

// Verify checks that the stored hash matches a recomputation.
func (b *Block) Verify() error {
	expected := ComputeHash(b.Index, b.Timestamp, b.Txs, b.Validator, b.PrevHash, b.Reward)
	if expected != b.Hash {
		return fmt.Errorf("block %d hash mismatch: got %s want %s", b.Index, b.Hash, expected)
	}
	return nil
}
