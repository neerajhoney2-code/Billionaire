package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DB is a simple file-backed key-value store backed by a JSON file per namespace.
// It is designed to be replaced with LevelDB or BadgerDB in production.
type DB struct {
	mu      sync.RWMutex
	dataDir string
	stores  map[string]map[string][]byte
}

// Open creates (or loads) a DB rooted at dataDir.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	db := &DB{
		dataDir: dataDir,
		stores:  make(map[string]map[string][]byte),
	}
	return db, nil
}

// namespace returns (creating if needed) the in-memory map for a given namespace.
func (db *DB) namespace(ns string) map[string][]byte {
	m, ok := db.stores[ns]
	if !ok {
		m = make(map[string][]byte)
		db.stores[ns] = m
		_ = db.loadNamespace(ns, m)
	}
	return m
}

func (db *DB) nsFile(ns string) string {
	return filepath.Join(db.dataDir, ns+".json")
}

func (db *DB) loadNamespace(ns string, m map[string][]byte) error {
	data, err := os.ReadFile(db.nsFile(ns))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m)
}

func (db *DB) saveNamespace(ns string, m map[string][]byte) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(db.nsFile(ns), data, 0o644)
}

// Put stores a JSON-marshalled value under ns:key.
func (db *DB) Put(ns, key string, value interface{}) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	m := db.namespace(ns)
	m[key] = raw
	return db.saveNamespace(ns, m)
}

// Get unmarshals the value for ns:key into dest.
func (db *DB) Get(ns, key string, dest interface{}) error {
	db.mu.RLock()
	defer db.mu.RUnlock()
	m := db.stores[ns]
	if m == nil {
		return fmt.Errorf("not found: %s/%s", ns, key)
	}
	raw, ok := m[key]
	if !ok {
		return fmt.Errorf("not found: %s/%s", ns, key)
	}
	return json.Unmarshal(raw, dest)
}

// Has returns true if the key exists in the namespace.
func (db *DB) Has(ns, key string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	m := db.stores[ns]
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// Delete removes a key.
func (db *DB) Delete(ns, key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	m := db.namespace(ns)
	delete(m, key)
	return db.saveNamespace(ns, m)
}

// Keys returns all keys in a namespace.
func (db *DB) Keys(ns string) []string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	m := db.stores[ns]
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Close is a no-op for the file store (all writes are immediate).
func (db *DB) Close() error { return nil }
