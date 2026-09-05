package syslogserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/hilather/go-lab-syslog/internal/syslogframing"
)

const (
	defaultTCPIdle     = 2 * time.Minute
	defaultMaxTCPConns = 256
	defaultMaxPerIP    = 16
)

func applyTCPDefaults(cfg Config) Config {
	cfg = applyDefaults(cfg)
	if cfg.Framing == "" {
		cfg.Framing = syslogframing.Auto
	}
	if cfg.TCPIdleTimeout <= 0 {
		cfg.TCPIdleTimeout = defaultTCPIdle
	}
	if cfg.SessionTimeout <= 0 {
		cfg.SessionTimeout = defaultTCPIdle
	}
	if cfg.MaxTCPConns <= 0 {
		cfg.MaxTCPConns = defaultMaxTCPConns
	}
	if cfg.MaxTCPConnsPerIP <= 0 {
		cfg.MaxTCPConnsPerIP = defaultMaxPerIP
	}
	return cfg
}

// ListenTCP binds net.Listen("tcp", cfg.Addr) and starts Accept.
func ListenTCP(ctx context.Context, cfg Config) (*Server, error) {
	cfg = applyTCPDefaults(cfg)
	switch cfg.Framing {
	case syslogframing.Auto, syslogframing.OctetCounting, syslogframing.NonTransparent:
	default:
		return nil, fmt.Errorf("tcp framing must be auto|octet-counting|non-transparent, got %q", cfg.Framing)
	}
	if cfg.Addr == "" {
		return nil, fmt.Errorf("tcp listen address is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{
		cfg:     cfg,
		ln:      ln,
		metrics: cfg.Metrics,
		ctx:     ctx,
		cancel:  cancel,
		conns:   make(map[net.Conn]netip.Addr),
		perIP:   make(map[netip.Addr]int),
	}
	s.PushLive(liveFromConfig(cfg))
	s.wg.Add(1)
	go s.serveTCP()
	return s, nil
}

func (s *Server) serveTCP() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if s.ctx.Err() != nil || isClosedConn(err) {
				return
			}
			continue
		}
		if !s.admitConn(conn) {
			_ = conn.Close()
			continue
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

// handleConn is the TCP call site: RFC 6587 split, then ingest. No parse here.
func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer s.releaseConn(conn)
	defer conn.Close()
	s.metrics.TCPConns.Add(1)
	defer s.metrics.TCPConns.Add(-1)

	remote, ok := tcpRemote(conn.RemoteAddr())
	if !ok {
		return
	}
	live := s.current()
	r := &deadlineReader{
		Conn:    conn,
		idle:    live.TCPIdleTimeout,
		session: time.Now().Add(live.SessionTimeout),
	}
	sc := syslogframing.NewScanner(r, live.Framing, live.MaxMessageBytes)
	for {
		if s.ctx.Err() != nil {
			return
		}
		frame, err := sc.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, syslogframing.ErrOversize) {
				s.metrics.drop(ReasonOversize)
				continue
			}
			if errors.Is(err, syslogframing.ErrFraming) {
				s.metrics.TCPFramingErrors.Add(1)
			}
			return
		}
		if len(frame) == 0 {
			s.metrics.drop(ReasonEmpty)
			continue
		}
		if s.ingest(s.ctx, TransportTCP, remote, frame) {
			return
		}
	}
}

func (s *Server) admitConn(conn net.Conn) bool {
	remote, ok := tcpRemote(conn.RemoteAddr())
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns == nil {
		s.conns = make(map[net.Conn]netip.Addr)
		s.perIP = make(map[netip.Addr]int)
	}
	live := s.current()
	if len(s.conns) >= live.MaxTCPConns {
		return false
	}
	if s.perIP[remote.ip] >= live.MaxTCPConnsPerIP {
		return false
	}
	s.conns[conn] = remote.ip
	s.perIP[remote.ip]++
	return true
}

func (s *Server) releaseConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ip, ok := s.conns[conn]
	if !ok {
		return
	}
	delete(s.conns, conn)
	s.perIP[ip]--
	if s.perIP[ip] <= 0 {
		delete(s.perIP, ip)
	}
}

func (s *Server) closeAllConns() {
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func tcpRemote(addr net.Addr) (remoteAddr, bool) {
	t, ok := addr.(*net.TCPAddr)
	if !ok || t == nil {
		return remoteAddr{}, false
	}
	ap := t.AddrPort()
	return remoteAddr{ip: ap.Addr().Unmap(), port: ap.Port()}, true
}

// deadlineReader refreshes the read deadline on each Read so idle is
// idle-since-last-byte, capped by sessionTimeout.
type deadlineReader struct {
	net.Conn
	idle    time.Duration
	session time.Time
}

func (c *deadlineReader) Read(p []byte) (int, error) {
	deadline := c.session
	if c.idle > 0 {
		idleAt := time.Now().Add(c.idle)
		if deadline.IsZero() || idleAt.Before(deadline) {
			deadline = idleAt
		}
	}
	if !deadline.IsZero() {
		if err := c.Conn.SetReadDeadline(deadline); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(p)
}
