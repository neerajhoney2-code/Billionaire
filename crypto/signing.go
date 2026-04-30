package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Verify returns true if signature (hex DER) over msg matches pubKeyHex (compressed P-256).
func Verify(pubKeyHex string, msg []byte, sigHex string) bool {
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false
	}
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubBytes)
	if x == nil {
		return false
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(msg)
	return ecdsa.VerifyASN1(pub, hash[:], sigBytes)
}

// PublicKeyFromPrivHex extracts the compressed public key hex from a DER private key hex.
func PublicKeyFromPrivHex(privHex string) (string, error) {
	w, err := FromPrivateKeyHex(privHex)
	if err != nil {
		return "", fmt.Errorf("load wallet: %w", err)
	}
	return w.PublicKey, nil
}

// HashBytes returns the hex SHA-256 of the input.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// AddressFromPub derives the blockchain address from a compressed public key hex.
func AddressFromPub(pubHex string) (string, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return "", err
	}
	x, _ := elliptic.UnmarshalCompressed(elliptic.P256(), b)
	if x == nil {
		return "", fmt.Errorf("invalid public key")
	}
	return deriveAddress(b), nil
}

// bigInt is only needed if we ever do raw r,s parsing — kept for future use.
var _ = big.NewInt
