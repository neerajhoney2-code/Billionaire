package consensus

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// SelectValidator picks a validator weighted by stake using the block's prevHash
// as a deterministic seed. This makes selection verifiable by all peers.
func (r *Registry) SelectValidator(prevHash string) (string, error) {
	active := r.ActiveValidators()
	if len(active) == 0 {
		return "", fmt.Errorf("no active validators")
	}

	// Derive a uint64 seed from the previous block hash.
	seed, err := hashToUint64(prevHash)
	if err != nil {
		return "", fmt.Errorf("compute seed: %w", err)
	}

	var totalStake uint64
	for _, v := range active {
		totalStake += v.Stake
	}
	if totalStake == 0 {
		return "", fmt.Errorf("total stake is zero")
	}

	// Pick a random point in [0, totalStake) using the seed.
	pick := seed % totalStake

	var cumulative uint64
	for _, v := range active {
		cumulative += v.Stake
		if pick < cumulative {
			return v.Address, nil
		}
	}

	// Fallback (floating-point safety): return last validator.
	return active[len(active)-1].Address, nil
}

// hashToUint64 reads the first 8 bytes of a hex hash string as big-endian uint64.
func hashToUint64(hexStr string) (uint64, error) {
	b, err := hex.DecodeString(hexStr)
	if err != nil || len(b) < 8 {
		return 0, fmt.Errorf("invalid hash: %s", hexStr)
	}
	return binary.BigEndian.Uint64(b[:8]), nil
}
