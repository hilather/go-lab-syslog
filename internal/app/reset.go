package app

import (
	"context"

	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

// Reset rereads bootstrap, compiles, swaps the snapshot, wipes store and
// audit, and rebinds listeners only when the effective address changed.
// The bootstrap file is never written. Compile failure keeps the live
// snapshot and returns bootstrap_invalid.
func (s *Service) Reset(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, err := config.LoadFile(s.cfg.BootstrapPath)
	if err != nil {
		return domainerr.Newf(domainerr.BootstrapInvalid, "bootstrap: %v", err)
	}
	snap, err := compiler.Compile(doc, s.cfg.Compiler)
	if err != nil {
		return domainerr.Newf(domainerr.BootstrapInvalid, "%s", err.Error())
	}

	if s.started {
		if ctx == nil {
			ctx = s.ctx
		}
		if err := s.bindLocked(ctx, snap); err != nil {
			return err
		}
	}

	s.snaps.Store(snap)
	if !s.started {
		s.pushLiveLocked(snap)
	}
	s.store.Wipe()
	s.audit.Wipe()
	s.idem = map[string]idemRecord{}
	s.audit.Append(audit.Event{
		Operation: audit.OpReset,
		Revision:  snap.Revision,
	})
	return nil
}
