package pharma

// Medicine represents a drug batch registered on the blockchain.
type Medicine struct {
	BatchID      string        `json:"batch_id"`
	Name         string        `json:"name"`
	Manufacturer string        `json:"manufacturer"` // blockchain address
	ManufDate    int64         `json:"manuf_date"`   // unix nano
	ExpiryDate   int64         `json:"expiry_date"`  // unix nano
	Composition  string        `json:"composition"`
	Quantity     uint64        `json:"quantity"`
	QRHash       string        `json:"qr_hash"`       // SHA-256 hash used as QR payload key
	RegisteredAt string        `json:"registered_at"` // block hash when confirmed
	SupplyChain  []SupplyStage `json:"supply_chain"`
	Status       Status        `json:"status"`
}

// Status tracks where in the supply chain the batch is.
type Status string

const (
	StatusManufactured Status = "manufactured"
	StatusDistributed  Status = "distributed"
	StatusRetail       Status = "retail"
	StatusDispensed    Status = "dispensed"
	StatusRecalled     Status = "recalled"
)

// SupplyStage records one hand-off in the supply chain.
type SupplyStage struct {
	Actor     string `json:"actor"`    // blockchain address
	Role      string `json:"role"`     // "manufacturer"|"distributor"|"retailer"|"patient"
	Timestamp int64  `json:"timestamp"`
	Location  string `json:"location"` // city / region
	TxID      string `json:"tx_id"`    // transaction that recorded this stage
}

// RegisterPayload is the JSON body inside a TxRegisterMed transaction's Data field.
type RegisterPayload struct {
	BatchID     string `json:"batch_id"`
	Name        string `json:"name"`
	ManufDate   int64  `json:"manuf_date"`
	ExpiryDate  int64  `json:"expiry_date"`
	Composition string `json:"composition"`
	Quantity    uint64 `json:"quantity"`
	Location    string `json:"location"`
}

// TransferPayload is the JSON body for TxTransferMed.
type TransferPayload struct {
	BatchID  string `json:"batch_id"`
	Recipient string `json:"recipient"` // next actor's address
	Role     string `json:"role"`      // "distributor" or "retailer"
	Location string `json:"location"`
}

// DispensePayload is the JSON body for TxDispenseMed.
type DispensePayload struct {
	BatchID  string `json:"batch_id"`
	Patient  string `json:"patient"` // can be anonymised hash
	Location string `json:"location"`
}

// RecallPayload is the JSON body for TxRecallMed.
type RecallPayload struct {
	BatchID string `json:"batch_id"`
	Reason  string `json:"reason"`
}
