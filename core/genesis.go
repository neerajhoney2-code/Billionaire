package core

import "time"

// GenesisAllocation specifies initial token grants.
type GenesisAllocation struct {
	Address string
	Amount  uint64
}

// DefaultAllocations are the initial token holders.
var DefaultAllocations = []GenesisAllocation{
	{Address: "genesis_treasury", Amount: 1_000_000_000},
}

// NewGenesisBlock creates block 0 with coinbase transactions.
func NewGenesisBlock(allocations []GenesisAllocation) *Block {
	txs := make([]*Transaction, 0, len(allocations))
	for _, a := range allocations {
		txs = append(txs, NewCoinbase(a.Address, a.Amount))
	}
	prevHash := "0000000000000000000000000000000000000000000000000000000000000000"
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	h := ComputeHash(0, ts, txs, "genesis", prevHash, 0)
	return &Block{
		Index:     0,
		Timestamp: ts,
		Txs:       txs,
		Validator: "genesis",
		PrevHash:  prevHash,
		Hash:      h,
	}
}
