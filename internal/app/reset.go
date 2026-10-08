package app

import (
	"context"

	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/observability"
)

// Reset rereads bootstrap, compiles, swaps the snapshot, and wipes store
// and audit. After Start, a management-address change is refused with
// validation_failed and does not swap, wipe, or rebind. When the
// management address is unchanged, a UDP or TCP socket whose address
// is unchanged stays bound; a changed address or a plane turned on
// listens again, and a plane turned off is closed.
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
	s.idemSeq = 0
	s.audit.Append(audit.Event{
		Actor:     actor,
		Operation: audit.OpReset,
		Reason:    reason,
		Revision:  snap.Revision,
	})
	observability.LogMutation("", audit.OpReset, "", snap.Revision)
	return nil
}
