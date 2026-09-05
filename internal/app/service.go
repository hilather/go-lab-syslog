package app

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"sync"

	"github.com/hilather/go-lab-syslog/internal/audit"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/snapshot"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

// Config is process wiring for one Service. Production code must not Dial.
type Config struct {
	BootstrapPath string
	Compiler      compiler.Options
}

// Service owns the snapshot, store, audit ring, and data-plane binds.
// Adapters call this type; it does not import net/http.
type Service struct {
	cfg      Config
	snaps    snapshot.Store
	store    *store.Store
	audit    *audit.Ring
	verifier *auth.Verifier
	sessions *auth.Store
	handler  syslogserver.Handler
	metrics  *syslogserver.Metrics

	mu       sync.Mutex
	started  bool
	ctx      context.Context
	udp      *syslogserver.Server
	tcp      *syslogserver.Server
	mgmt     net.Listener
	udpAddr  string
	tcpAddr  string
	mgmtAddr string

	idem map[string]idemRecord
}

type idemRecord struct {
	fingerprint string
	result      ApplyResult
	err         error
}

// New loads bootstrap, compiles a snapshot, and constructs store + audit.
// Listeners are bound in Start.
func New(cfg Config) (*Service, error) {
	if cfg.BootstrapPath == "" {
		return nil, domainerr.New(domainerr.ValidationFailed, "--config is required")
	}
	if cfg.Compiler.ConfigDir == "" {
		cfg.Compiler.ConfigDir = filepath.Dir(cfg.BootstrapPath)
	}
	doc, err := config.LoadFile(cfg.BootstrapPath)
	if err != nil {
		return nil, err
	}
	snap, err := compiler.Compile(doc, cfg.Compiler)
	if err != nil {
		return nil, err
	}
	st := store.NewFromSpec(snap.Document.Spec.Store)
	ver, err := auth.FromSpec(snap.Document.Spec.Auth, cfg.Compiler.ConfigDir)
	if err != nil {
		return nil, err
	}
	s := &Service{
		cfg:      cfg,
		store:    st,
		audit:    audit.New(snap.Document.Spec.Observability.Audit.Ring),
		verifier: ver,
		sessions: auth.NewStore(auth.DefaultSessionConfig()),
		metrics:  &syslogserver.Metrics{},
		idem:     map[string]idemRecord{},
	}
	s.handler = syslogserver.HandlerFunc(func(_ context.Context, msg model.Message) error {
		_, err := s.store.Insert(msg)
		return err
	})
	s.snaps.Store(snap)
	return s, nil
}

// Start binds enabled UDP/TCP and, when requested, a management TCP listener
// (REST is mounted by cmd/labsyslog on ManagementListener).
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.ctx = ctx
	if err := s.bindLocked(ctx, s.snaps.Load()); err != nil {
		s.closeListenersLocked()
		return err
	}
	s.started = true
	return nil
}

// Close stops listeners. Idempotent.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeListenersLocked()
}

func (s *Service) closeListenersLocked() error {
	var first error
	if s.udp != nil {
		if err := s.udp.Close(); err != nil && first == nil {
			first = err
		}
		s.udp = nil
	}
	if s.tcp != nil {
		if err := s.tcp.Close(); err != nil && first == nil {
			first = err
		}
		s.tcp = nil
	}
	if s.mgmt != nil {
		if err := s.mgmt.Close(); err != nil && first == nil {
			first = err
		}
		s.mgmt = nil
	}
	s.started = false
	return first
}

// Snapshot is the live compiled document. Never mutate it.
func (s *Service) Snapshot() *snapshot.Snapshot {
	return s.snaps.Load()
}

// Messages is the ephemeral store. The insert Handler is this store.
func (s *Service) Messages() *store.Store { return s.store }

// AuditRing is the mutation log.
func (s *Service) AuditRing() *audit.Ring { return s.audit }

// Verifier is the compiled bearer index. REST and MCP share this pointer.
func (s *Service) Verifier() *auth.Verifier {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.verifier
}

// Sessions is the REST-only cookie table. MCP must not use it.
func (s *Service) Sessions() *auth.Store { return s.sessions }

// Metrics is the shared ingest counters (UDP and TCP).
func (s *Service) Metrics() *syslogserver.Metrics { return s.metrics }

// UDPAddr is the bound UDP address, or nil.
func (s *Service) UDPAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.udp == nil {
		return nil
	}
	return s.udp.LocalAddr()
}

// TCPAddr is the bound TCP syslog address, or nil.
func (s *Service) TCPAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tcp == nil {
		return nil
	}
	return s.tcp.LocalAddr()
}

// ManagementListener is the bound management socket, or nil. REST is mounted
// by cmd/labsyslog; this package must not import net/http.
func (s *Service) ManagementListener() net.Listener {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mgmt
}

// ManagementAddr is the bound management address, or empty when off.
func (s *Service) ManagementAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mgmt == nil {
		return ""
	}
	return s.mgmt.Addr().String()
}

// Ready is true when the snapshot and store exist, every enabled data-plane
// listener is bound, and management is bound or was not requested.
func (s *Service) Ready() bool {
	snap := s.snaps.Load()
	if snap == nil || s.store == nil {
		return false
	}
	spec := snap.Document.Spec
	udpOn, _ := listenerOn(spec.Listeners.UDP.Enabled, spec.Listeners.UDP.Address)
	tcpOn, _ := listenerOn(spec.Listeners.TCP.Enabled, spec.Listeners.TCP.Address)
	mgmtOn := spec.Listeners.Management.Address != ""

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return false
	}
	if udpOn && s.udp == nil {
		return false
	}
	if tcpOn && s.tcp == nil {
		return false
	}
	if mgmtOn && s.mgmt == nil {
		return false
	}
	return true
}

// DeleteMessage removes one stored message and audits the mutation.
func (s *Service) DeleteMessage(_ context.Context, id, actor, reason string) error {
	if err := s.store.Delete(id); err != nil {
		return err
	}
	rev := ""
	if snap := s.snaps.Load(); snap != nil {
		rev = snap.Revision
	}
	s.audit.Append(audit.Event{
		Actor:     actor,
		Operation: audit.OpDelete,
		Reason:    reason,
		Revision:  rev,
	})
	return nil
}

// ClearMessages wipes the store (waiters see store_wiped) and audits.
func (s *Service) ClearMessages(_ context.Context, actor, reason string) {
	s.store.Clear()
	rev := ""
	if snap := s.snaps.Load(); snap != nil {
		rev = snap.Revision
	}
	s.audit.Append(audit.Event{
		Actor:     actor,
		Operation: audit.OpClear,
		Reason:    reason,
		Revision:  rev,
	})
}

// Validate checks a candidate without requiring token files to exist.
func (s *Service) Validate(_ context.Context, doc model.Document) error {
	d := compiler.CloneDocument(doc)
	return compiler.Check(&d, s.cfg.Compiler.ConfigDir)
}

// Export returns canonical YAML of the live snapshot.
func (s *Service) Export(_ context.Context) ([]byte, error) {
	snap := s.snaps.Load()
	if snap == nil {
		return nil, domainerr.New(domainerr.ValidationFailed, "snapshot is empty")
	}
	return bytes.Clone(snap.Canonical), nil
}

// State returns the redacted live spec, revision, generation, and drifted.
func (s *Service) State(_ context.Context) StateView {
	snap := s.snaps.Load()
	view := StateView{}
	if snap != nil {
		view.Document = compiler.CloneDocument(snap.Document)
		view.Revision = snap.Revision
		view.Drifted = s.drifted(snap)
	}
	if s.store != nil {
		view.Generation = s.store.Stats().Generation
	}
	return view
}

func (s *Service) drifted(snap *snapshot.Snapshot) bool {
	if snap == nil {
		return true
	}
	boot, err := config.LoadFile(s.cfg.BootstrapPath)
	if err != nil {
		return true
	}
	if err := compiler.Check(boot, s.cfg.Compiler.ConfigDir); err != nil {
		return true
	}
	canon, err := config.EncodeCanonical(boot)
	if err != nil {
		return true
	}
	return !bytes.Equal(snap.Canonical, canon)
}

func (s *Service) bindLocked(ctx context.Context, snap *snapshot.Snapshot) error {
	if snap == nil {
		return domainerr.New(domainerr.ValidationFailed, "snapshot is empty")
	}
	spec := snap.Document.Spec
	udpOn, udpAddr := listenerOn(spec.Listeners.UDP.Enabled, spec.Listeners.UDP.Address)
	tcpOn, tcpAddr := listenerOn(spec.Listeners.TCP.Enabled, spec.Listeners.TCP.Address)
	mgmtAddr := spec.Listeners.Management.Address
	mgmtOn := mgmtAddr != ""

	if !udpOn && !tcpOn {
		return domainerr.New(domainerr.ValidationFailed, "no data-plane listener enabled")
	}

	cfg, err := s.ingestConfig(snap)
	if err != nil {
		return err
	}

	// Bind every new socket first. Drain old listeners only after all listens
	// succeed so a later plane failure cannot leave new sockets + old snapshot.
	type pending struct {
		udp, tcp *syslogserver.Server
		mgmt     net.Listener
		dropUDP  bool
		dropTCP  bool
		dropMgmt bool
		udpAddr  string
		tcpAddr  string
		mgmtAddr string
	}
	var p pending
	rollback := true
	defer func() {
		if !rollback {
			return
		}
		if p.udp != nil {
			_ = p.udp.Close()
		}
		if p.tcp != nil {
			_ = p.tcp.Close()
		}
		if p.mgmt != nil {
			_ = p.mgmt.Close()
		}
	}()

	if udpOn {
		if s.udp == nil || s.udpAddr != udpAddr {
			cfg.Addr = udpAddr
			next, err := syslogserver.ListenUDP(ctx, cfg)
			if err != nil {
				return err
			}
			p.udp = next
			p.udpAddr = udpAddr
		}
	} else if s.udp != nil {
		p.dropUDP = true
	}
	if tcpOn {
		if s.tcp == nil || s.tcpAddr != tcpAddr {
			cfg.Addr = tcpAddr
			next, err := syslogserver.ListenTCP(ctx, cfg)
			if err != nil {
				return err
			}
			p.tcp = next
			p.tcpAddr = tcpAddr
		}
	} else if s.tcp != nil {
		p.dropTCP = true
	}
	if mgmtOn {
		if s.mgmt == nil || s.mgmtAddr != mgmtAddr {
			ln, err := net.Listen("tcp", mgmtAddr)
			if err != nil {
				return err
			}
			p.mgmt = ln
			p.mgmtAddr = mgmtAddr
		}
	} else if s.mgmt != nil {
		p.dropMgmt = true
	}

	rollback = false
	if p.udp != nil {
		if s.udp != nil {
			_ = s.udp.Close()
		}
		s.udp = p.udp
		s.udpAddr = p.udpAddr
	} else if p.dropUDP {
		_ = s.udp.Close()
		s.udp = nil
		s.udpAddr = ""
	}
	if p.tcp != nil {
		if s.tcp != nil {
			_ = s.tcp.Close()
		}
		s.tcp = p.tcp
		s.tcpAddr = p.tcpAddr
	} else if p.dropTCP {
		_ = s.tcp.Close()
		s.tcp = nil
		s.tcpAddr = ""
	}
	if p.mgmt != nil {
		if s.mgmt != nil {
			_ = s.mgmt.Close()
		}
		s.mgmt = p.mgmt
		s.mgmtAddr = p.mgmtAddr
	} else if p.dropMgmt {
		_ = s.mgmt.Close()
		s.mgmt = nil
		s.mgmtAddr = ""
	}
	s.pushLiveLocked(snap)
	return nil
}

func listenerOn(enabled *bool, addr string) (bool, string) {
	on := enabled == nil || *enabled
	return on && addr != "", addr
}

func (s *Service) ingestConfig(snap *snapshot.Snapshot) (syslogserver.Config, error) {
	spec := snap.Document.Spec
	admit, err := syslogserver.NewCIDRAdmission(spec.Admission)
	if err != nil {
		return syslogserver.Config{}, err
	}
	class, err := syslogserver.NewFilterClassifier(spec.Filters)
	if err != nil {
		return syslogserver.Config{}, err
	}
	return syslogserver.Config{
		UDPMaxDatagramBytes: int(spec.Syslog.UDPMaxDatagramBytes),
		MaxMessageBytes:     int(spec.Syslog.MaxMessageBytes),
		Framing:             spec.Listeners.TCP.Framing,
		TCPIdleTimeout:      spec.Syslog.TCPIdleTimeout.Duration(),
		SessionTimeout:      spec.Admission.SessionTimeout.Duration(),
		MaxTCPConns:         spec.Admission.MaxTCPConns,
		MaxTCPConnsPerIP:    spec.Admission.MaxTCPConnsPerIP,
		Parse:               syslogwire.OptionsFromParse(spec.Syslog.Parse),
		Admission:           admit,
		Classifier:          class,
		Behavior:            syslogserver.Behavior{Mode: spec.Syslog.Behavior.Mode},
		Handler:             s.handler,
		Metrics:             s.metrics,
	}, nil
}

func (s *Service) pushLiveLocked(snap *snapshot.Snapshot) {
	if snap == nil {
		return
	}
	spec := snap.Document.Spec
	admit, err := syslogserver.NewCIDRAdmission(spec.Admission)
	if err != nil {
		return
	}
	class, err := syslogserver.NewFilterClassifier(spec.Filters)
	if err != nil {
		return
	}
	live := syslogserver.Live{
		Admission:           admit,
		Classifier:          class,
		Behavior:            syslogserver.Behavior{Mode: spec.Syslog.Behavior.Mode},
		Parse:               syslogwire.OptionsFromParse(spec.Syslog.Parse),
		MaxMessageBytes:     int(spec.Syslog.MaxMessageBytes),
		UDPMaxDatagramBytes: int(spec.Syslog.UDPMaxDatagramBytes),
		MaxTCPConns:         spec.Admission.MaxTCPConns,
		MaxTCPConnsPerIP:    spec.Admission.MaxTCPConnsPerIP,
		SessionTimeout:      spec.Admission.SessionTimeout.Duration(),
		Framing:             spec.Listeners.TCP.Framing,
		TCPIdleTimeout:      spec.Syslog.TCPIdleTimeout.Duration(),
	}
	if s.udp != nil {
		s.udp.PushLive(live)
	}
	if s.tcp != nil {
		s.tcp.PushLive(live)
	}
	s.store.ApplyCaps(store.ConfigFromSpec(spec.Store))
	s.audit.Resize(spec.Observability.Audit.Ring)
}
