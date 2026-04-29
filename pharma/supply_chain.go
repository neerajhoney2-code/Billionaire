package pharma

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/neerajhoney2-code/billionaire/crypto"
)

// Registry manages all registered medicines in memory (persisted separately via storage.DB).
type Registry struct {
	mu        sync.RWMutex
	medicines map[string]*Medicine // keyed by BatchID
	byQRHash  map[string]string    // QRHash → BatchID
}

func NewRegistry() *Registry {
	return &Registry{
		medicines: make(map[string]*Medicine),
		byQRHash:  make(map[string]string),
	}
}

// Register adds a new medicine batch from a RegisterPayload.
func (r *Registry) Register(p RegisterPayload, manufacturer string, txID, blockHash string) (*Medicine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.medicines[p.BatchID]; exists {
		return nil, fmt.Errorf("batch %s already registered", p.BatchID)
	}

	qrHash := computeQRHash(p.BatchID, manufacturer, p.ManufDate)
	med := &Medicine{
		BatchID:      p.BatchID,
		Name:         p.Name,
		Manufacturer: manufacturer,
		ManufDate:    p.ManufDate,
		ExpiryDate:   p.ExpiryDate,
		Composition:  p.Composition,
		Quantity:     p.Quantity,
		QRHash:       qrHash,
		RegisteredAt: blockHash,
		Status:       StatusManufactured,
		SupplyChain: []SupplyStage{
			{
				Actor:     manufacturer,
				Role:      "manufacturer",
				Timestamp: time.Now().UnixNano(),
				Location:  p.Location,
				TxID:      txID,
			},
		},
	}
	r.medicines[p.BatchID] = med
	r.byQRHash[qrHash] = p.BatchID
	return med, nil
}

// Transfer records a custody transfer in the supply chain.
func (r *Registry) Transfer(p TransferPayload, sender string, txID string) (*Medicine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	med, ok := r.medicines[p.BatchID]
	if !ok {
		return nil, fmt.Errorf("batch %s not found", p.BatchID)
	}
	if med.Status == StatusRecalled {
		return nil, fmt.Errorf("batch %s has been recalled", p.BatchID)
	}
	if med.Status == StatusDispensed {
		return nil, fmt.Errorf("batch %s already dispensed", p.BatchID)
	}

	switch p.Role {
	case "distributor":
		med.Status = StatusDistributed
	case "retailer":
		med.Status = StatusRetail
	default:
		return nil, fmt.Errorf("unknown role: %s", p.Role)
	}

	med.SupplyChain = append(med.SupplyChain, SupplyStage{
		Actor:     p.Recipient,
		Role:      p.Role,
		Timestamp: time.Now().UnixNano(),
		Location:  p.Location,
		TxID:      txID,
	})
	return med, nil
}

// Dispense records the final patient dispensing step.
func (r *Registry) Dispense(p DispensePayload, sender string, txID string) (*Medicine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	med, ok := r.medicines[p.BatchID]
	if !ok {
		return nil, fmt.Errorf("batch %s not found", p.BatchID)
	}
	if med.Status == StatusRecalled {
		return nil, fmt.Errorf("batch %s recalled", p.BatchID)
	}
	med.Status = StatusDispensed
	med.SupplyChain = append(med.SupplyChain, SupplyStage{
		Actor:     p.Patient,
		Role:      "patient",
		Timestamp: time.Now().UnixNano(),
		Location:  p.Location,
		TxID:      txID,
	})
	return med, nil
}

// Recall marks a batch as recalled.
func (r *Registry) Recall(p RecallPayload, txID string) (*Medicine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	med, ok := r.medicines[p.BatchID]
	if !ok {
		return nil, fmt.Errorf("batch %s not found", p.BatchID)
	}
	med.Status = StatusRecalled
	med.SupplyChain = append(med.SupplyChain, SupplyStage{
		Actor:     "recall_authority",
		Role:      "recall",
		Timestamp: time.Now().UnixNano(),
		Location:  "regulatory",
		TxID:      txID,
	})
	return med, nil
}

// GetByBatchID returns a medicine by its batch ID.
func (r *Registry) GetByBatchID(batchID string) (*Medicine, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.medicines[batchID]
	if !ok {
		return nil, false
	}
	cp := *m
	return &cp, true
}

// GetByQRHash looks up a medicine by its QR hash.
func (r *Registry) GetByQRHash(qrHash string) (*Medicine, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	batchID, ok := r.byQRHash[qrHash]
	if !ok {
		return nil, false
	}
	m := r.medicines[batchID]
	cp := *m
	return &cp, true
}

// All returns all registered medicines.
func (r *Registry) All() []*Medicine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Medicine, 0, len(r.medicines))
	for _, m := range r.medicines {
		cp := *m
		out = append(out, &cp)
	}
	return out
}

// ApplyTxData processes the Data field of a pharma transaction.
func (r *Registry) ApplyTxData(txType string, sender string, data []byte, txID, blockHash string) error {
	switch txType {
	case "register_medicine":
		var p RegisterPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		_, err := r.Register(p, sender, txID, blockHash)
		return err
	case "transfer_medicine":
		var p TransferPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		_, err := r.Transfer(p, sender, txID)
		return err
	case "dispense_medicine":
		var p DispensePayload
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		_, err := r.Dispense(p, sender, txID)
		return err
	case "recall_medicine":
		var p RecallPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return err
		}
		_, err := r.Recall(p, txID)
		return err
	}
	return nil
}

// computeQRHash creates the stable QR identifier for a medicine batch.
func computeQRHash(batchID, manufacturer string, manufDate int64) string {
	raw := fmt.Sprintf("%s|%s|%d", batchID, manufacturer, manufDate)
	return crypto.HashBytes([]byte(raw))
}
