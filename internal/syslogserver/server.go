package syslogserver

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

const defaultCap = 64 * model.KiB

// Config is a UDP/TCP listener plus ingest stubs. FIL-001 replaces Admission,
// Classifier, and Behavior; STORE-001/FIL-001 replace Handler.
type Config struct {
	Addr                string
	UDPMaxDatagramBytes int
	MaxMessageBytes     int
	Framing             string
	TCPIdleTimeout      time.Duration
	SessionTimeout      time.Duration
	MaxTCPConns         int
	MaxTCPConnsPerIP    int
	Parse               syslogwire.Options
	Admission           Admission
	Classifier          Classifier
	Behavior            Behavior
	Handler             Handler
	Now                 func() time.Time
	Metrics             *Metrics
}

// Server is one ingest pipeline with an optional UDP PacketConn and/or TCP Listener.
type Server struct {
	cfg       Config
	pc        net.PacketConn
	ln        net.Listener
	metrics   *Metrics
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error

	mu    sync.Mutex
	conns map[net.Conn]netip.Addr
	perIP map[netip.Addr]int
}

func applyDefaults(cfg Config) Config {
	if cfg.Handler == nil {
		cfg.Handler = NopHandler{}
	}
	if cfg.Admission == nil {
		cfg.Admission = AllowAll{}
	}
	if cfg.Classifier == nil {
		cfg.Classifier = CaptureAll{}
	}
	if cfg.Behavior.Mode == "" {
		cfg.Behavior.Mode = BehaviorAccept
	}
	if cfg.UDPMaxDatagramBytes <= 0 {
		cfg.UDPMaxDatagramBytes = int(defaultCap)
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = int(defaultCap)
	}
	if !cfg.Parse.RFC3164 && !cfg.Parse.RFC5424 {
		cfg.Parse = syslogwire.DefaultOptions()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = &Metrics{}
	}
	return cfg
}

func (c Config) effectiveCap() int {
	a, b := c.UDPMaxDatagramBytes, c.MaxMessageBytes
	if a < b {
		return a
	}
	return b
}

// ListenUDP binds net.ListenPacket("udp", cfg.Addr) and starts the read loop.
func ListenUDP(ctx context.Context, cfg Config) (*Server, error) {
	cfg = applyDefaults(cfg)
	if cfg.Addr == "" {
		return nil, fmt.Errorf("udp listen address is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pc, err := net.ListenPacket("udp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		cfg:     cfg,
		pc:      pc,
		metrics: cfg.Metrics,
		ctx:     ctx,
		cancel:  cancel,
	}
	s.wg.Add(1)
	go s.serveUDP()
	return s, nil
}

// LocalAddr is the bound UDP address, or the TCP address when there is no UDP conn.
func (s *Server) LocalAddr() net.Addr {
	if s == nil {
		return nil
	}
	if s.pc != nil {
		return s.pc.LocalAddr()
	}
	if s.ln != nil {
		return s.ln.Addr()
	}
	return nil
}

// Metrics returns ingest counters. Never nil after ListenUDP or ListenTCP.
func (s *Server) Metrics() *Metrics { return s.metrics }

// Close stops the UDP loop and TCP accept/sessions. Idempotent.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.pc != nil {
			s.closeErr = s.pc.Close()
		}
		if s.ln != nil {
			if err := s.ln.Close(); s.closeErr == nil {
				s.closeErr = err
			}
		}
		s.closeAllConns()
		s.wg.Wait()
	})
	return s.closeErr
}
