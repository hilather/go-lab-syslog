package syslogserver

import (
	"bytes"
	"context"
	"net/netip"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

type remoteAddr struct {
	ip   netip.Addr
	port uint16
}

// ingest runs admission → behavior.mode → parse → classify → Handler.Insert.
// Callers apply size/framing first and must not parse.
func (s *Server) ingest(ctx context.Context, transport string, remote remoteAddr, raw []byte) {
	if ctx == nil {
		ctx = s.ctx
	}
	remote.ip = remote.ip.Unmap()

	ok, reason := s.cfg.Admission.Allow(remote.ip)
	if !ok {
		s.metrics.drop(normalizeAdmissionReason(reason))
		return
	}
	s.metrics.Received.Add(1)

	switch s.cfg.Behavior.Mode {
	case BehaviorDropSilent, BehaviorClose:
		s.metrics.drop(ReasonBehavior)
		return
	}

	now := s.now()
	opts := s.cfg.Parse
	if opts.Now.IsZero() {
		opts.Now = now
	}
	parsed, warn, err := syslogwire.Parse(raw, opts)
	if err != nil {
		s.metrics.drop(ReasonUnparseable)
		return
	}

	msg := model.Message{
		ReceivedAt:   now.UTC(),
		Transport:    transport,
		RemoteIP:     remote.ip,
		RemotePort:   remote.port,
		Raw:          bytes.Clone(raw),
		Truncated:    false,
		ParseWarning: warn,
		Parsed:       parsed,
	}

	action, tag := s.cfg.Classifier.Classify(&msg)
	switch action {
	case ActionDropSilent:
		s.metrics.drop(ReasonFilter)
		return
	case ActionTag:
		if tag != "" {
			msg.Tags = append(msg.Tags, tag)
		}
	}

	if err := s.cfg.Handler.Insert(ctx, msg); err != nil {
		s.metrics.drop(ReasonStoreFull)
		return
	}
	s.metrics.Stored.Add(1)
}

func (s *Server) now() time.Time {
	if s.cfg.Now != nil {
		return s.cfg.Now()
	}
	return time.Now()
}

func normalizeAdmissionReason(reason string) string {
	switch reason {
	case ReasonAdmission, ReasonAdmissionRate:
		return reason
	default:
		return ReasonAdmission
	}
}
