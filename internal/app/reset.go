package app

import (
	"context"

	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

// Reset rereads bootstrap, compiles, swaps the snapshot, wipes store and
// audit, and rebinds listeners only when the effective address changed.
// The bootstrap file is never written. Compile failure keeps the live
// snapshot and returns bootstrap_invalid. Sessions are dropped with the
// audit ring.
func (s *Service) Reset(ctx context.Context, actor, reason string) error {
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
	ver, err := auth.FromSpec(snap.Document.Spec.Auth, s.cfg.Compiler.ConfigDir)
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
	s.verifier = ver
	if !s.started {
		s.pushLiveLocked(snap)
	}
	s.store.Wipe()
	s.audit.Wipe()
	if s.sessions != nil {
		s.sessions.Clear()
	}
	s.idem = map[string]idemRecord{}
	s.audit.Append(audit.Event{
		Actor:     actor,
		Operation: audit.OpReset,
		Reason:    reason,
		Revision:  snap.Revision,
	})
	return nil
}
