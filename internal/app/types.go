package app

import (
	"github.com/hilather/go-lab-syslog/internal/model"
)

// Closed live operations (docs/04). Anything else is reset-only.
const (
	OpReplaceStoreCaps     = "replaceStoreCaps"
	OpReplaceFilters       = "replaceFilters"
	OpReplaceAdmission     = "replaceAdmission"
	OpReplaceSyslogParse   = "replaceSyslogParse"
	OpReplaceBehavior      = "replaceBehavior"
	OpReplaceObservability = "replaceObservability"
	OpReplaceListeners     = "replaceListeners"
)

// Operation is one coarse replace in a plan/apply body.
type Operation struct {
	Type            string             `json:"type"`
	Store           *model.Store       `json:"store,omitempty"`
	Filters         []model.Filter     `json:"filters,omitempty"`
	Admission       *model.Admission   `json:"admission,omitempty"`
	Parse           *model.SyslogParse `json:"parse,omitempty"`
	MaxMessageBytes *model.ByteSize    `json:"maxMessageBytes,omitempty"`
	Behavior        *model.Behavior    `json:"behavior,omitempty"`
	LogLevel        string             `json:"logLevel,omitempty"`
	Fields          []string           `json:"fields,omitempty"`
}

// PlanRequest is changes:plan. Operations are applied as given; Candidate
// (when Operations is empty) is diffed against the live snapshot.
type PlanRequest struct {
	ExpectedRevision string          `json:"expectedRevision"`
	Operations       []Operation     `json:"operations,omitempty"`
	Candidate        *model.Document `json:"candidate,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	Actor            string          `json:"actor,omitempty"`
}

// ApplyRequest is changes:apply. IdempotencyKey is required.
type ApplyRequest struct {
	ExpectedRevision string          `json:"expectedRevision"`
	Operations       []Operation     `json:"operations,omitempty"`
	Candidate        *model.Document `json:"candidate,omitempty"`
	Reason           string          `json:"reason,omitempty"`
	Actor            string          `json:"actor,omitempty"`
	IdempotencyKey   string          `json:"idempotencyKey"`
}

// Plan is the dry-run result.
type Plan struct {
	ExpectedRevision string      `json:"expectedRevision"`
	NextRevision     string      `json:"nextRevision"`
	Operations       []Operation `json:"operations"`
	Immutable        bool        `json:"immutable,omitempty"`
}

// ApplyResult is the committed (or replayed) apply outcome.
type ApplyResult struct {
	Revision         string `json:"revision"`
	Plan             Plan   `json:"plan"`
	IdempotentReplay bool   `json:"idempotentReplay,omitempty"`
}

// StateView is GET /v1/state without HTTP.
type StateView struct {
	Document   model.Document `json:"spec"`
	Revision   string         `json:"revision"`
	Generation uint64         `json:"generation"`
	Drifted    bool           `json:"drifted"`
}
