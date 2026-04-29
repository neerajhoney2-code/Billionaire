package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/neerajhoney2-code/billionaire/consensus"
	"github.com/neerajhoney2-code/billionaire/pharma"
	"github.com/neerajhoney2-code/billionaire/rewards"
)

// Mempool is a thread-safe pool of pending transactions.
type Mempool struct {
	mu      sync.Mutex
	pending map[string]*Transaction
}

func NewMempool() *Mempool {
	return &Mempool{pending: make(map[string]*Transaction)}
}

// Add validates and inserts a transaction. Returns an error if rejected.
func (m *Mempool) Add(tx *Transaction) error {
	if tx.ID == "" {
		return fmt.Errorf("transaction has no ID")
	}
	if tx.Type != TxCoinbase && !tx.Verify() {
		return fmt.Errorf("invalid signature on tx %s", tx.ID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.pending[tx.ID]; exists {
		return fmt.Errorf("duplicate tx %s", tx.ID)
	}
	m.pending[tx.ID] = tx
	return nil
}

// Consume removes and returns up to max transactions.
func (m *Mempool) Consume(max int) []*Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Transaction, 0, max)
	for id, tx := range m.pending {
		out = append(out, tx)
		delete(m.pending, id)
		if len(out) >= max {
			break
		}
	}
	return out
}

// Return puts transactions back (e.g. if block forging failed).
func (m *Mempool) Return(txs []*Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, tx := range txs {
		m.pending[tx.ID] = tx
	}
}

// All returns a snapshot of all pending transactions.
func (m *Mempool) All() []*Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Transaction, 0, len(m.pending))
	for _, tx := range m.pending {
		out = append(out, tx)
	}
	return out
}

// Size returns pending tx count.
func (m *Mempool) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

// -----------------------------------------------------------------------

// Blockchain is the main chain orchestrator.
type Blockchain struct {
	mu         sync.RWMutex
	chain      []*Block
	state      *State
	validators *consensus.Registry
	slasher    *consensus.SlashTracker
	mempool    *Mempool
	pharmaReg  *pharma.Registry

	// Public keys known to this node: address → pubKey hex.
	pubKeys map[string]string
}

// New creates a blockchain with the default genesis block.
func New() *Blockchain {
	bc := &Blockchain{
		chain:      make([]*Block, 0),
		state:      NewState(),
		validators: consensus.NewRegistry(),
		slasher:    consensus.NewSlashTracker(),
		mempool:    NewMempool(),
		pharmaReg:  pharma.NewRegistry(),
		pubKeys:    make(map[string]string),
	}

	genesis := NewGenesisBlock(DefaultAllocations)
	bc.applyBlock(genesis)
	bc.chain = append(bc.chain, genesis)
	return bc
}

// Mempool exposes the mempool for the API layer.
func (bc *Blockchain) Mempool() *Mempool { return bc.mempool }

// Validators exposes the registry.
func (bc *Blockchain) Validators() *consensus.Registry { return bc.validators }

// PharmaRegistry exposes the pharma registry.
func (bc *Blockchain) PharmaRegistry() *pharma.Registry { return bc.pharmaReg }

// State exposes account state.
func (bc *Blockchain) State() *State { return bc.state }

// Height returns the index of the last block.
func (bc *Blockchain) Height() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.chain[len(bc.chain)-1].Index
}

// LastBlock returns the most recent block.
func (bc *Blockchain) LastBlock() *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.chain[len(bc.chain)-1]
}

// GetBlock returns the block at the given index.
func (bc *Blockchain) GetBlock(index uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if index >= uint64(len(bc.chain)) {
		return nil, fmt.Errorf("block %d not found", index)
	}
	return bc.chain[index], nil
}

// RecentBlocks returns the last n blocks (or all if n > chain length).
func (bc *Blockchain) RecentBlocks(n int) []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	start := len(bc.chain) - n
	if start < 0 {
		start = 0
	}
	return bc.chain[start:]
}

// Chain returns a copy of the full chain slice.
func (bc *Blockchain) Chain() []*Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	out := make([]*Block, len(bc.chain))
	copy(out, bc.chain)
	return out
}

// RegisterPublicKey stores a public key → address mapping.
func (bc *Blockchain) RegisterPublicKey(address, pubKey string) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.pubKeys[address] = pubKey
}

// SubmitTransaction validates and adds a tx to the mempool.
func (bc *Blockchain) SubmitTransaction(tx *Transaction) error {
	// Basic validation
	if tx.Amount == 0 && tx.Type == TxTransfer {
		return fmt.Errorf("transfer amount must be > 0")
	}
	if tx.Type != TxCoinbase {
		// Check nonce
		expectedNonce := bc.state.Nonce(tx.Sender)
		if tx.Nonce != expectedNonce {
			return fmt.Errorf("invalid nonce: expected %d got %d", expectedNonce, tx.Nonce)
		}
		// Balance check for value transfers
		if tx.Type == TxTransfer || tx.Type == TxStake {
			total := tx.Amount + tx.Fee
			if bc.state.Balance(tx.Sender) < total {
				return fmt.Errorf("insufficient balance: need %d have %d", total, bc.state.Balance(tx.Sender))
			}
		}
	}
	return bc.mempool.Add(tx)
}

// StakeTokens stakes amount for address and registers the validator.
func (bc *Blockchain) StakeTokens(address, pubKey string, amount uint64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.state.AddStake(address, amount); err != nil {
		return err
	}
	currentStake := bc.state.Stake(address)
	if _, err := bc.validators.Register(address, pubKey, currentStake); err != nil {
		// Roll back stake change
		_ = bc.state.RemoveStake(address, amount)
		return err
	}
	bc.pubKeys[address] = pubKey
	return nil
}

// UnstakeTokens removes stake and updates the validator registry.
func (bc *Blockchain) UnstakeTokens(address string, amount uint64) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if err := bc.state.RemoveStake(address, amount); err != nil {
		return err
	}
	newStake := bc.state.Stake(address)
	if _, err := bc.validators.RemoveStake(address, amount); err != nil {
		_ = bc.state.AddStake(address, amount)
		return err
	}
	_ = newStake
	return nil
}

// ForgeBlock selects a validator, builds a block, and appends it.
func (bc *Blockchain) ForgeBlock() (*Block, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	last := bc.chain[len(bc.chain)-1]

	validatorAddr, err := bc.validators.SelectValidator(last.Hash)
	if err != nil {
		return nil, fmt.Errorf("select validator: %w", err)
	}

	txs := bc.mempool.Consume(100)

	// Validate txs against a clone of state (prevents double-spend within block).
	stateClone := bc.state.Clone()
	valid := make([]*Transaction, 0, len(txs))
	invalid := make([]*Transaction, 0)

	for _, tx := range txs {
		if err := validateTxAgainstState(tx, stateClone); err != nil {
			invalid = append(invalid, tx)
			continue
		}
		applyTxToState(tx, stateClone)
		valid = append(valid, tx)
	}
	// Return invalid txs to mempool.
	bc.mempool.Return(invalid)

	reward := rewards.BlockReward(last.Index + 1)
	ts := time.Now().UnixNano()
	hash := ComputeHash(last.Index+1, ts, valid, validatorAddr, last.Hash, reward)

	block := &Block{
		Index:     last.Index + 1,
		Timestamp: ts,
		Txs:       valid,
		Validator: validatorAddr,
		PrevHash:  last.Hash,
		Hash:      hash,
		Reward:    reward,
	}

	// Sign the block if we have the validator's wallet key (P2P nodes sign their own blocks).
	// For now, store empty signature — the validator's identity is verified via the registry.

	if err := bc.appendBlock(block); err != nil {
		return nil, err
	}
	return block, nil
}

// AppendBlock validates and appends an externally received block (P2P).
func (bc *Blockchain) AppendBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.appendBlock(block)
}

func (bc *Blockchain) appendBlock(block *Block) error {
	last := bc.chain[len(bc.chain)-1]

	// Structural validation.
	if block.Index != last.Index+1 {
		return fmt.Errorf("block index mismatch: want %d got %d", last.Index+1, block.Index)
	}
	if block.PrevHash != last.Hash {
		return fmt.Errorf("block prev_hash mismatch")
	}
	if err := block.Verify(); err != nil {
		if block.Validator != "genesis" {
			burned, _ := bc.validators.Slash(block.Validator)
			bc.state.SlashStake(block.Validator, burned)
		}
		return fmt.Errorf("block hash invalid: %w", err)
	}
	if block.Validator != "genesis" && !bc.validators.IsRegistered(block.Validator) {
		return fmt.Errorf("block validator %s not registered", block.Validator)
	}

	// Slash detection.
	if block.Validator != "genesis" {
		if _, err := bc.slasher.RecordSigned(block.Validator, block.Index, block.Hash); err != nil {
			burned, _ := bc.validators.Slash(block.Validator)
			bc.state.SlashStake(block.Validator, burned)
			return fmt.Errorf("slashing validator: %w", err)
		}
	}

	// Apply all state changes from this block.
	bc.applyBlock(block)

	bc.chain = append(bc.chain, block)
	bc.validators.RecordBlock(block.Validator)
	return nil
}

// applyBlock applies the state changes of a block without appending to the chain.
func (bc *Blockchain) applyBlock(block *Block) {
	for _, tx := range block.Txs {
		applyTxToState(tx, bc.state)
		// Apply pharma operations.
		if isPharmaType(tx.Type) {
			_ = bc.pharmaReg.ApplyTxData(string(tx.Type), tx.Sender, tx.Data, tx.ID, block.Hash)
		}
	}
	// Credit block reward to validator.
	if block.Validator != "genesis" && block.Reward > 0 {
		bc.state.Credit(block.Validator, block.Reward)
	}
}

// ValidateChain performs a full integrity scan.
func (bc *Blockchain) ValidateChain() (bool, string) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for i := 1; i < len(bc.chain); i++ {
		b := bc.chain[i]
		prev := bc.chain[i-1]
		if err := b.Verify(); err != nil {
			return false, fmt.Sprintf("block %d: %v", b.Index, err)
		}
		if b.PrevHash != prev.Hash {
			return false, fmt.Sprintf("block %d prev_hash broken", b.Index)
		}
		for _, tx := range b.Txs {
			if !tx.Verify() {
				return false, fmt.Sprintf("block %d tx %s: invalid signature", b.Index, tx.ID)
			}
		}
	}
	return true, "ok"
}

// ReplaceChain replaces the chain if newChain is longer and valid.
func (bc *Blockchain) ReplaceChain(newChain []*Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if len(newChain) <= len(bc.chain) {
		return fmt.Errorf("incoming chain not longer than current")
	}
	// Verify the new chain before replacing.
	for i := 1; i < len(newChain); i++ {
		b := newChain[i]
		prev := newChain[i-1]
		if err := b.Verify(); err != nil {
			return fmt.Errorf("incoming block %d invalid: %w", b.Index, err)
		}
		if b.PrevHash != prev.Hash {
			return fmt.Errorf("incoming chain broken at block %d", b.Index)
		}
	}
	// Reset and replay.
	bc.chain = newChain
	bc.state = NewState()
	for _, block := range bc.chain {
		bc.applyBlock(block)
	}
	return nil
}

// FindTransaction searches all blocks for a tx by ID.
func (bc *Blockchain) FindTransaction(txID string) (*Transaction, uint64, bool) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	for _, block := range bc.chain {
		for _, tx := range block.Txs {
			if tx.ID == txID {
				return tx, block.Index, true
			}
		}
	}
	return nil, 0, false
}

// ---- helpers ----

func validateTxAgainstState(tx *Transaction, s *State) error {
	if tx.Type == TxCoinbase {
		return nil
	}
	if !tx.Verify() {
		return fmt.Errorf("invalid signature")
	}
	if tx.Type == TxTransfer {
		total := tx.Amount + tx.Fee
		if s.Balance(tx.Sender) < total {
			return fmt.Errorf("insufficient balance")
		}
	}
	if tx.Type == TxStake {
		if s.Balance(tx.Sender) < tx.Amount {
			return fmt.Errorf("insufficient balance for stake")
		}
	}
	return nil
}

func applyTxToState(tx *Transaction, s *State) {
	switch tx.Type {
	case TxCoinbase:
		s.Credit(tx.Recipient, tx.Amount)
	case TxTransfer:
		_ = s.Debit(tx.Sender, tx.Amount+tx.Fee)
		s.Credit(tx.Recipient, tx.Amount)
		s.IncrementNonce(tx.Sender)
	case TxStake:
		_ = s.AddStake(tx.Sender, tx.Amount)
		s.IncrementNonce(tx.Sender)
	case TxUnstake:
		_ = s.RemoveStake(tx.Sender, tx.Amount)
		s.IncrementNonce(tx.Sender)
	case TxRegisterMed, TxTransferMed, TxDispenseMed, TxRecallMed:
		if tx.Fee > 0 {
			_ = s.Debit(tx.Sender, tx.Fee)
		}
		s.IncrementNonce(tx.Sender)
	}
}

func isPharmaType(t TxType) bool {
	switch t {
	case TxRegisterMed, TxTransferMed, TxDispenseMed, TxRecallMed:
		return true
	}
	return false
}

// MarshalJSON serialises the whole blockchain for P2P sync.
func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return json.Marshal(bc.chain)
}
