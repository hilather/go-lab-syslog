package auth

import (
	"crypto/sha256"
	"crypto/subtle"
)

// MinTokenBytes is the fail-closed minimum length of a trimmed bearer token.
const MinTokenBytes = 32

// DigestSecret hashes secret bytes for constant-time comparison.
func DigestSecret(secret []byte) [sha256.Size]byte {
	return sha256.Sum256(secret)
}

// EqualDigest reports whether two SHA-256 digests are equal using
// crypto/subtle so the comparison does not short-circuit.
func EqualDigest(a, b [sha256.Size]byte) bool {
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
