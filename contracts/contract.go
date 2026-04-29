package contracts

import (
	"fmt"
	"sync"

	"github.com/neerajhoney2-code/billionaire/crypto"
)

// Contract holds deployed bytecode and its storage.
type Contract struct {
	Address  string            `json:"address"`
	Deployer string            `json:"deployer"`
	Code     []byte            `json:"code"`
	Storage  map[string]uint64 `json:"storage"`
}

// Registry manages deployed contracts.
type Registry struct {
	mu        sync.RWMutex
	contracts map[string]*Contract // address → contract
}

func NewRegistry() *Registry {
	return &Registry{contracts: make(map[string]*Contract)}
}

// Deploy stores a new contract and returns its address.
// Address is derived from deployer + code hash.
func (r *Registry) Deploy(deployer string, code []byte) (*Contract, error) {
	if len(code) == 0 {
		return nil, fmt.Errorf("contract code cannot be empty")
	}
	addr := crypto.HashBytes(append([]byte(deployer), code...))[:40]

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.contracts[addr]; exists {
		return nil, fmt.Errorf("contract already deployed at %s", addr)
	}
	c := &Contract{
		Address:  addr,
		Deployer: deployer,
		Code:     code,
		Storage:  make(map[string]uint64),
	}
	r.contracts[addr] = c
	return c, nil
}

// Call executes a contract, mutating its storage. Returns the result value.
func (r *Registry) Call(address string, gas uint64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.contracts[address]
	if !ok {
		return 0, fmt.Errorf("contract %s not found", address)
	}

	vm := NewVM(c.Code, c.Storage, gas)
	result, err := vm.Execute()
	if err != nil {
		return 0, fmt.Errorf("execution failed: %w", err)
	}
	// Persist mutated storage.
	c.Storage = vm.Storage()
	return result, nil
}

// Get returns a contract by address.
func (r *Registry) Get(address string) (*Contract, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.contracts[address]
	if !ok {
		return nil, false
	}
	return c, true
}
