// Package security contains small crypto helpers shared across features.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
)

// GenerateNumericCode returns a uniformly random digit string of the given length.
func GenerateNumericCode(length int) (string, error) {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%0*d", length, n), nil
}

// NewSalt returns a random hex salt.
func NewSalt() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashSecret hashes a short secret (e.g. an OTP) with a per-record salt so the
// plain value never has to be stored.
func HashSecret(salt, secret string) string {
	sum := sha256.Sum256([]byte(salt + ":" + secret))
	return hex.EncodeToString(sum[:])
}

// SecretMatches compares in constant time.
func SecretMatches(salt, secret, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashSecret(salt, secret)), []byte(hash)) == 1
}
