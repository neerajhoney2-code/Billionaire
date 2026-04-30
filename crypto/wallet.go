package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
)

// Wallet holds an ECDSA key pair and the derived blockchain address.
type Wallet struct {
	privateKey *ecdsa.PrivateKey
	Address    string
	PublicKey  string // hex-encoded compressed public key bytes
}

// Generate creates a new random wallet.
func Generate() (*Wallet, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return newWallet(priv), nil
}

// FromPrivateKeyHex reconstructs a wallet from a hex-encoded DER private key.
func FromPrivateKeyHex(privHex string) (*Wallet, error) {
	b, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	priv, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}
	return newWallet(priv), nil
}

func newWallet(priv *ecdsa.PrivateKey) *Wallet {
	pubBytes := elliptic.MarshalCompressed(priv.Curve, priv.PublicKey.X, priv.PublicKey.Y)
	pubHex := hex.EncodeToString(pubBytes)
	addr := deriveAddress(pubBytes)
	return &Wallet{privateKey: priv, PublicKey: pubHex, Address: addr}
}

// PrivateKeyHex returns the DER-encoded private key as a hex string.
func (w *Wallet) PrivateKeyHex() (string, error) {
	b, err := x509.MarshalECPrivateKey(w.privateKey)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// PrivateKeyPEM returns the PEM-encoded private key.
func (w *Wallet) PrivateKeyPEM() (string, error) {
	b, err := x509.MarshalECPrivateKey(w.privateKey)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	return string(pem.EncodeToMemory(block)), nil
}

// Sign signs the SHA-256 hash of msg and returns a hex-encoded DER signature.
func (w *Wallet) Sign(msg []byte) (string, error) {
	hash := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, w.privateKey, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return hex.EncodeToString(sig), nil
}

// deriveAddress produces a 40-char hex address from a compressed public key.
func deriveAddress(pubBytes []byte) string {
	h := sha256.Sum256(pubBytes)
	return hex.EncodeToString(h[:])[:40]
}

// DeriveAddress is the exported version.
func DeriveAddress(pubHex string) (string, error) {
	b, err := hex.DecodeString(pubHex)
	if err != nil {
		return "", err
	}
	return deriveAddress(b), nil
}
