package snapshot

import "github.com/hilather/go-lab-syslog/internal/model"

// Snapshot is an immutable compiled document plus its revision.
// Compile-to-snapshot lives in internal/compiler (STA-001).
type Snapshot struct {
	Document  model.Document
	Canonical []byte
	Revision  string
}
