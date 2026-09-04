package compiler

import (
	"crypto/sha256"
	"encoding/hex"
)

// Revision is sha256: plus lowercase hex of canonical YAML bytes.
func Revision(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:])
}
