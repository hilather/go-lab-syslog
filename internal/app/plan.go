package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/snapshot"
)

var liveOps = map[string]bool{
	OpReplaceStoreCaps:     true,
	OpReplaceFilters:       true,
	OpReplaceAdmission:     true,
	OpReplaceSyslogParse:   true,
	OpReplaceBehavior:      true,
	OpReplaceObservability: true,
}

// Plan is a dry-run of operations (or a candidate diff). It does not mutate.
func (s *Service) Plan(_ context.Context, req PlanRequest) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cand, err := s.prepareLocked(req.ExpectedRevision, req.Operations, req.Candidate)
	if err != nil {
		return Plan{}, err
	}
	return cand.plan, nil
}

// Apply commits live operations. Duplicate Idempotency-Key + identical body
// returns the original result. Reset-only fields are immutable_field.
func (s *Service) Apply(_ context.Context, req ApplyRequest) (ApplyResult, error) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return ApplyResult{}, domainerr.New(domainerr.ValidationFailed, "idempotencyKey is required")
	}
	fp, err := applyFingerprint(req)
	if err != nil {
		return ApplyResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if rec, ok := s.idem[req.IdempotencyKey]; ok {
		if rec.fingerprint == fp {
			out := rec.result
			out.IdempotentReplay = true
			return out, rec.err
		}
		return ApplyResult{}, domainerr.New(domainerr.IdempotencyConflict, "idempotency key reused with a different body")
	}

	cand, err := s.prepareLocked(req.ExpectedRevision, req.Operations, req.Candidate)
	if err != nil {
		return ApplyResult{}, err
	}
	if cand.plan.Immutable {
		return ApplyResult{}, domainerr.Newf(domainerr.ImmutableField, "reset-only field %s", immutableDetail(cand.plan.Operations))
	}

	s.snaps.Store(cand.next)
	s.pushLiveLocked(cand.next)
	s.audit.Append(audit.Event{
		Actor:     req.Actor,
		Operation: audit.OpApply,
		Reason:    req.Reason,
		Revision:  cand.next.Revision,
	})
	out := ApplyResult{Revision: cand.next.Revision, Plan: cand.plan}
	s.idem[req.IdempotencyKey] = idemRecord{fingerprint: fp, result: out}
	return out, nil
}

type prepared struct {
	plan Plan
	next *snapshot.Snapshot
}

func (s *Service) prepareLocked(expected string, ops []Operation, candidate *model.Document) (prepared, error) {
	cur := s.snaps.Load()
	if cur == nil {
		return prepared{}, domainerr.New(domainerr.ValidationFailed, "snapshot is empty")
	}
	if expected != cur.Revision {
		return prepared{}, domainerr.New(domainerr.RevisionMismatch, "expectedRevision does not match live snapshot")
	}

	var err error
	if len(ops) == 0 && candidate != nil {
		ops, err = diffOperations(cur.Document, *candidate, s.cfg.Compiler.ConfigDir)
		if err != nil {
			return prepared{}, err
		}
	}

	nextDoc := compiler.CloneDocument(cur.Document)
	if err := applyOperations(&nextDoc.Spec, ops); err != nil {
		return prepared{}, err
	}
	next, err := compiler.Compile(&nextDoc, compiler.Options{ConfigDir: s.cfg.Compiler.ConfigDir})
	if err != nil {
		return prepared{}, err
	}
	plan := Plan{
		ExpectedRevision: expected,
		NextRevision:     next.Revision,
		Operations:       ops,
		Immutable:        hasImmutable(ops),
	}
	return prepared{plan: plan, next: next}, nil
}

func applyOperations(spec *model.Spec, ops []Operation) error {
	for i, op := range ops {
		switch op.Type {
		case OpReplaceStoreCaps:
			if op.Store == nil {
				return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: replaceStoreCaps requires store", i)
			}
			spec.Store = *op.Store
		case OpReplaceFilters:
			spec.Filters = compiler.CloneDocument(model.Document{Spec: model.Spec{Filters: op.Filters}}).Spec.Filters
			if spec.Filters == nil {
				spec.Filters = []model.Filter{}
			}
		case OpReplaceAdmission:
			if op.Admission == nil {
				return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: replaceAdmission requires admission", i)
			}
			spec.Admission = *op.Admission
		case OpReplaceSyslogParse:
			if op.Parse != nil {
				spec.Syslog.Parse = *op.Parse
			}
			if op.MaxMessageBytes != nil {
				spec.Syslog.MaxMessageBytes = *op.MaxMessageBytes
			}
		case OpReplaceBehavior:
			if op.Behavior == nil || op.Behavior.Mode == "" {
				return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: replaceBehavior requires behavior.mode", i)
			}
			spec.Syslog.Behavior = *op.Behavior
		case OpReplaceObservability:
			if op.LogLevel == "" {
				return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: replaceObservability requires logLevel", i)
			}
			spec.Observability.LogLevel = op.LogLevel
		case OpReplaceListeners:
			// recorded for plan; apply rejects via Immutable
		default:
			if op.Type == "" {
				return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: type is required", i)
			}
			return domainerr.Newf(domainerr.ValidationFailed, "operations[%d]: unknown type %q", i, op.Type)
		}
	}
	return nil
}

func hasImmutable(ops []Operation) bool {
	for _, op := range ops {
		if !liveOps[op.Type] {
			return true
		}
	}
	return false
}

func immutableDetail(ops []Operation) string {
	for _, op := range ops {
		if liveOps[op.Type] {
			continue
		}
		if len(op.Fields) > 0 {
			return strings.Join(op.Fields, ", ")
		}
		return op.Type
	}
	return OpReplaceListeners
}

func diffOperations(current, candidate model.Document, configDir string) ([]Operation, error) {
	cand := compiler.CloneDocument(candidate)
	if err := compiler.Check(&cand, configDir); err != nil {
		return nil, err
	}
	cur := current.Spec
	next := cand.Spec
	var ops []Operation
	if !reflect.DeepEqual(cur.Store, next.Store) {
		st := next.Store
		ops = append(ops, Operation{Type: OpReplaceStoreCaps, Store: &st})
	}
	if !reflect.DeepEqual(cur.Filters, next.Filters) {
		ops = append(ops, Operation{Type: OpReplaceFilters, Filters: next.Filters})
	}
	if !reflect.DeepEqual(cur.Admission, next.Admission) {
		ad := next.Admission
		ops = append(ops, Operation{Type: OpReplaceAdmission, Admission: &ad})
	}
	if !reflect.DeepEqual(cur.Syslog.Parse, next.Syslog.Parse) || cur.Syslog.MaxMessageBytes != next.Syslog.MaxMessageBytes {
		p := next.Syslog.Parse
		b := next.Syslog.MaxMessageBytes
		ops = append(ops, Operation{Type: OpReplaceSyslogParse, Parse: &p, MaxMessageBytes: &b})
	}
	if cur.Syslog.Behavior != next.Syslog.Behavior {
		b := next.Syslog.Behavior
		ops = append(ops, Operation{Type: OpReplaceBehavior, Behavior: &b})
	}
	if cur.Observability.LogLevel != next.Observability.LogLevel {
		ops = append(ops, Operation{Type: OpReplaceObservability, LogLevel: next.Observability.LogLevel})
	}

	var fields []string
	if !reflect.DeepEqual(cur.Listeners, next.Listeners) {
		if cur.Listeners.UDP.Address != next.Listeners.UDP.Address {
			fields = append(fields, "spec.listeners.udp.address")
		} else {
			fields = append(fields, "spec.listeners")
		}
	}
	if !reflect.DeepEqual(cur.Auth, next.Auth) {
		fields = append(fields, "spec.auth")
	}
	if !reflect.DeepEqual(cur.UI, next.UI) {
		fields = append(fields, "spec.ui.enabled")
	}
	if !reflect.DeepEqual(cur.Management, next.Management) {
		if cur.Management.BodyLimit != next.Management.BodyLimit {
			fields = append(fields, "spec.management.bodyLimit")
		} else {
			fields = append(fields, "spec.management")
		}
	}
	if cur.Syslog.Hostname != next.Syslog.Hostname {
		fields = append(fields, "spec.syslog.hostname")
	}
	if cur.Syslog.UDPMaxDatagramBytes != next.Syslog.UDPMaxDatagramBytes {
		fields = append(fields, "spec.syslog.udpMaxDatagramBytes")
	}
	if cur.Syslog.TCPIdleTimeout != next.Syslog.TCPIdleTimeout {
		fields = append(fields, "spec.syslog.tcpIdleTimeout")
	}
	if cur.Observability.Metrics != next.Observability.Metrics {
		fields = append(fields, "spec.observability.metrics.publicPath")
	}
	if cur.Observability.Audit != next.Observability.Audit {
		fields = append(fields, "spec.observability.audit.ring")
	}
	if len(fields) > 0 {
		ops = append(ops, Operation{Type: OpReplaceListeners, Fields: fields})
	}
	return ops, nil
}

func applyFingerprint(req ApplyRequest) (string, error) {
	body := struct {
		ExpectedRevision string          `json:"expectedRevision"`
		Operations       []Operation     `json:"operations,omitempty"`
		Candidate        *model.Document `json:"candidate,omitempty"`
		Reason           string          `json:"reason,omitempty"`
	}{req.ExpectedRevision, req.Operations, req.Candidate, req.Reason}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
