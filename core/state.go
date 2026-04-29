package core

import (
	"fmt"
	"sync"
)

// Account represents an address's on-chain state.
type Account struct {
	Balance uint64
	Nonce   uint64
	Stake   uint64
}

// Spendable returns the balance minus staked tokens.
func (a *Account) Spendable() uint64 {
	if a.Stake >= a.Balance {
		return 0
	}
	return a.Balance - a.Stake
}

// State is the in-memory world state: all accounts.
type State struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

func NewState() *State {
	return &State{accounts: make(map[string]*Account)}
}

func (s *State) get(addr string) *Account {
	a, ok := s.accounts[addr]
	if !ok {
		a = &Account{}
		s.accounts[addr] = a
	}
	return a
}

// Balance returns the spendable balance for addr.
func (s *State) Balance(addr string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[addr]
	if !ok {
		return 0
	}
	return a.Spendable()
}

// TotalBalance returns the full balance (including staked).
func (s *State) TotalBalance(addr string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[addr]
	if !ok {
		return 0
	}
	return a.Balance
}

// Stake returns staked amount.
func (s *State) Stake(addr string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accounts[addr].Stake
}

// Nonce returns current nonce.
func (s *State) Nonce(addr string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[addr]
	if !ok {
		return 0
	}
	return a.Nonce
}

// Credit adds amount to addr's balance.
func (s *State) Credit(addr string, amount uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.get(addr).Balance += amount
}

// Debit subtracts amount from addr's spendable balance.
func (s *State) Debit(addr string, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.get(addr)
	if a.Spendable() < amount {
		return fmt.Errorf("insufficient balance: %s has %d spendable, need %d", addr, a.Spendable(), amount)
	}
	a.Balance -= amount
	return nil
}

// AddStake moves amount from spendable into staked for addr.
func (s *State) AddStake(addr string, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.get(addr)
	if a.Spendable() < amount {
		return fmt.Errorf("insufficient spendable balance to stake: have %d need %d", a.Spendable(), amount)
	}
	a.Stake += amount
	return nil
}

// RemoveStake moves amount from staked back to spendable for addr.
func (s *State) RemoveStake(addr string, amount uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.get(addr)
	if a.Stake < amount {
		return fmt.Errorf("insufficient stake: have %d need %d", a.Stake, amount)
	}
	a.Stake -= amount
	return nil
}

// SlashStake burns amount from staked (does not return to balance).
func (s *State) SlashStake(addr string, amount uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.get(addr)
	if amount > a.Stake {
		amount = a.Stake
	}
	a.Stake -= amount
	a.Balance -= amount
}

// IncrementNonce bumps the nonce by 1.
func (s *State) IncrementNonce(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.get(addr).Nonce++
}

// AllAccounts returns a snapshot of all accounts.
func (s *State) AllAccounts() map[string]Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Account, len(s.accounts))
	for addr, a := range s.accounts {
		out[addr] = *a
	}
	return out
}

// Clone returns a deep copy for temporary validation.
func (s *State) Clone() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	clone := NewState()
	for addr, a := range s.accounts {
		ac := *a
		clone.accounts[addr] = &ac
	}
	return clone
}
