package syslogserver

import (
	"context"
	"net/netip"

	"github.com/hilather/go-lab-syslog/internal/model"
)

// TransportUDP is Message.Transport for RFC 5426 datagrams.
const TransportUDP = "udp"

// TransportTCP is Message.Transport for RFC 6587 streams.
const TransportTCP = "tcp"

// Classifier actions (docs/04).
const (
	ActionCapture    = "capture"
	ActionDropSilent = "drop-silent"
	ActionTag        = "tag"
)

// Behavior modes (docs/02). UDP close is drop-silent.
const (
	BehaviorAccept     = "accept"
	BehaviorDropSilent = "drop-silent"
	BehaviorClose      = "close"
)

// Drop reasons match docs/09 label values where those exist.
const (
	ReasonOversize      = "oversize"
	ReasonEmpty         = "empty"
	ReasonAdmission     = "admission_cidr"
	ReasonAdmissionRate = "admission_rate"
	ReasonBehavior      = "behavior"
	ReasonFilter        = "filter"
	ReasonUnparseable   = "unparseable"
	ReasonStoreFull     = "store_full"
)

// Handler receives a parsed, classified message.
type Handler interface {
	Insert(ctx context.Context, msg model.Message) error
}

// Admission is the first policy gate after size/framing.
type Admission interface {
	Allow(remote netip.Addr) (ok bool, reason string)
}

// Classifier is first-match capture/drop/tag after parse.
type Classifier interface {
	Classify(msg *model.Message) (action string, tag string)
}

// Behavior is process-wide ingest mode.
type Behavior struct {
	Mode string
}

// NopHandler discards Insert.
type NopHandler struct{}

func (NopHandler) Insert(context.Context, model.Message) error { return nil }

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, model.Message) error

func (f HandlerFunc) Insert(ctx context.Context, msg model.Message) error {
	return f(ctx, msg)
}

// AllowAll admits every remote. Tests that do not exercise CIDR use it.
type AllowAll struct{}

func (AllowAll) Allow(netip.Addr) (bool, string) { return true, "" }

// CaptureAll classifies every message as capture.
type CaptureAll struct{}

func (CaptureAll) Classify(*model.Message) (string, string) { return ActionCapture, "" }
