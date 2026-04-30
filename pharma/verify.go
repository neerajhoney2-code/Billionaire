package pharma

import "time"

// VerificationResult is returned when a QR code is scanned.
type VerificationResult struct {
	Genuine     bool      `json:"genuine"`
	Message     string    `json:"message"`
	Medicine    *Medicine `json:"medicine,omitempty"`
	ScannedAt   time.Time `json:"scanned_at"`
	VerifyURL   string    `json:"verify_url"`
}

// Verify looks up a medicine by QR hash and returns the result.
func (r *Registry) Verify(qrHash string) *VerificationResult {
	med, ok := r.GetByQRHash(qrHash)
	now := time.Now()

	if !ok {
		return &VerificationResult{
			Genuine:   false,
			Message:   "COUNTERFEIT — This medicine is NOT registered on the blockchain. Do not consume.",
			ScannedAt: now,
		}
	}

	if med.Status == StatusRecalled {
		return &VerificationResult{
			Genuine:   false,
			Message:   "RECALLED — This batch has been recalled. Do not consume.",
			Medicine:  med,
			ScannedAt: now,
		}
	}

	// Check expiry.
	if med.ExpiryDate > 0 && now.UnixNano() > med.ExpiryDate {
		return &VerificationResult{
			Genuine:   true,
			Message:   "GENUINE but EXPIRED — This medicine is authentic but past its expiry date.",
			Medicine:  med,
			ScannedAt: now,
		}
	}

	return &VerificationResult{
		Genuine:   true,
		Message:   "GENUINE — This medicine is verified on the blockchain.",
		Medicine:  med,
		ScannedAt: now,
	}
}
