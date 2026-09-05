package syslogserver

import (
	"context"
	"net/netip"

	"github.com/hilather/go-lab-syslog/internal/model"
)

// TransportUDP is Message.Transport for RFC 5426 datagrams.
const TransportUDP = "udp"

// Classifier actions (docs/04). FIL-001 fills real first-match.
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

// Handler receives a parsed, classified message. STORE-001 implements Insert;
// FIL-001 wires store.Store (or an adapter) from serve.
type Handler interface {
	Insert(ctx context.Context, msg model.Message) error
}

// Admission is the first policy gate after size/framing. UDP-001 stubs allow-all.
type Admission interface {
	Allow(remote netip.Addr) (ok bool, reason string)
}

// Classifier is first-match capture/drop/tag after parse. UDP-001 stubs capture-all.
type Classifier interface {
	Classify(msg *model.Message) (action string, tag string)
}

// Behavior is process-wide ingest mode. UDP-001 stubs accept.
type Behavior struct {
	Mode string
}

// NopHandler discards Insert. Serve uses it until FIL-001 wires the store.
type NopHandler struct{}

func (NopHandler) Insert(context.Context, model.Message) error { return nil }

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, model.Message) error

func (f HandlerFunc) Insert(ctx context.Context, msg model.Message) error {
	return f(ctx, msg)
}

// AllowAll is the UDP-001 admission stub.
type AllowAll struct{}

func (AllowAll) Allow(netip.Addr) (bool, string) { return true, "" }

// CaptureAll is the UDP-001 classifier stub.
type CaptureAll struct{}

func (CaptureAll) Classify(*model.Message) (string, string) { return ActionCapture, "" }
