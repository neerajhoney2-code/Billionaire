package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/neerajhoney2-code/billionaire/config"
)

// ValidatorInfo holds one validator's registration data.
type ValidatorInfo struct {
	Address       string    `json:"address"`
	PublicKey     string    `json:"public_key"`
	Stake         uint64    `json:"stake"`
	RegisteredAt  time.Time `json:"registered_at"`
	BlocksProduced int      `json:"blocks_produced"`
	SlashCount    int       `json:"slash_count"`
	Active        bool      `json:"active"`
}

// Registry manages the set of active PoS validators.
type Registry struct {
	mu         sync.RWMutex
	validators map[string]*ValidatorInfo
}

// NewRegistry creates an empty validator registry.
func NewRegistry() *Registry {
	return &Registry{validators: make(map[string]*ValidatorInfo)}
}

// Register adds or updates a validator. The amount here is what the caller
// has already locked in state; Registry just records it.
func (r *Registry) Register(address, publicKey string, stake uint64) (*ValidatorInfo, error) {
	if stake < config.MinStake {
		return nil, fmt.Errorf("stake %d below minimum %d", stake, config.MinStake)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	v, ok := r.validators[address]
	if !ok {
		v = &ValidatorInfo{
			Address:      address,
			PublicKey:    publicKey,
			RegisteredAt: time.Now(),
		}
		r.validators[address] = v
	}
	v.Stake = stake
	v.Active = true
	return v, nil
}

// AddStake increases a validator's recorded stake.
func (r *Registry) AddStake(address string, additional uint64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.validators[address]
	if !ok {
		return 0, fmt.Errorf("validator %s not registered", address)
	}
	v.Stake += additional
	return v.Stake, nil
}

// RemoveStake decreases a validator's stake; deactivates if below minimum.
func (r *Registry) RemoveStake(address string, amount uint64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.validators[address]
	if !ok {
		return 0, fmt.Errorf("validator %s not registered", address)
	}
	if amount > v.Stake {
		return 0, fmt.Errorf("cannot remove %d stake, only have %d", amount, v.Stake)
	}
	v.Stake -= amount
	if v.Stake < config.MinStake {
		v.Active = false
	}
	return v.Stake, nil
}

// Slash burns slashFraction of the validator's stake and records the event.
func (r *Registry) Slash(address string) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.validators[address]
	if !ok {
		return 0, fmt.Errorf("validator %s not found", address)
	}
	burned := uint64(float64(v.Stake) * config.SlashPercent)
	if burned == 0 {
		burned = 1
	}
	if burned > v.Stake {
		burned = v.Stake
	}
	v.Stake -= burned
	v.SlashCount++
	if v.Stake < config.MinStake {
		v.Active = false
	}
	return burned, nil
}

// RecordBlock increments a validator's block counter.
func (r *Registry) RecordBlock(address string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v, ok := r.validators[address]; ok {
		v.BlocksProduced++
	}
}

// IsRegistered returns true if addr is an active validator.
func (r *Registry) IsRegistered(address string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.validators[address]
	return ok && v.Active
}

// Get returns a copy of the validator info.
func (r *Registry) Get(address string) (ValidatorInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.validators[address]
	if !ok {
		return ValidatorInfo{}, false
	}
	return *v, true
}

// All returns a snapshot of all validators (active and inactive).
func (r *Registry) All() []ValidatorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ValidatorInfo, 0, len(r.validators))
	for _, v := range r.validators {
		out = append(out, *v)
	}
	return out
}

// ActiveValidators returns only active validators.
func (r *Registry) ActiveValidators() []ValidatorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []ValidatorInfo
	for _, v := range r.validators {
		if v.Active {
			out = append(out, *v)
		}
	}
	return out
}

// TotalStake returns the sum of all active validators' stakes.
func (r *Registry) TotalStake() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var total uint64
	for _, v := range r.validators {
		if v.Active {
			total += v.Stake
		}
	}
	return total
}
