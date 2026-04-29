package consensus

import (
	"fmt"
	"sync"
)

// DoubleSignEvidence records a validator signing two different blocks at the
// same height — the canonical slashable offence.
type DoubleSignEvidence struct {
	Validator string
	Height    uint64
	Hash1     string
	Hash2     string
}

// SlashTracker keeps track of signed block hashes per validator per height.
type SlashTracker struct {
	mu      sync.Mutex
	records map[string]map[uint64]string // validator → height → hash
}

func NewSlashTracker() *SlashTracker {
	return &SlashTracker{records: make(map[string]map[uint64]string)}
}

// RecordSigned notes that validator signed blockHash at height.
// Returns DoubleSignEvidence if this is a second different hash at the same height.
func (st *SlashTracker) RecordSigned(validator string, height uint64, blockHash string) (*DoubleSignEvidence, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.records[validator] == nil {
		st.records[validator] = make(map[uint64]string)
	}
	prev, exists := st.records[validator][height]
	if exists && prev != blockHash {
		return &DoubleSignEvidence{
			Validator: validator,
			Height:    height,
			Hash1:     prev,
			Hash2:     blockHash,
		}, fmt.Errorf("double sign detected for validator %s at height %d", validator, height)
	}
	st.records[validator][height] = blockHash
	return nil, nil
}
