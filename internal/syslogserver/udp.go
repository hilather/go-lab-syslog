package syslogserver

import (
	"bytes"
	"errors"
	"net"
)

// maxUDPPayload is the IPv4/IPv6 UDP length ceiling. A larger read buffer
// cannot observe a larger datagram.
const maxUDPPayload = 65535

func (s *Server) serveUDP() {
	defer s.wg.Done()
	limit := s.cfg.effectiveCap()
	if limit > maxUDPPayload {
		limit = maxUDPPayload
	}
	buf := make([]byte, limit+1)
	for {
		n, addr, err := s.pc.ReadFrom(buf)
		if err != nil {
			if s.ctx.Err() != nil || isClosedConn(err) {
				return
			}
			continue
		}
		s.handleDatagram(addr, buf[:n])
	}
}

// handleDatagram is the UDP call site: one datagram = one message, no parse.
func (s *Server) handleDatagram(addr net.Addr, payload []byte) {
	if len(payload) == 0 {
		s.metrics.drop(ReasonEmpty)
		return
	}
	if len(payload) > s.cfg.UDPMaxDatagramBytes || len(payload) > s.cfg.MaxMessageBytes {
		s.metrics.drop(ReasonOversize)
		return
	}
	remote, ok := udpRemote(addr)
	if !ok {
		// ListenPacket yields *UDPAddr; any other net.Addr is not a CIDR miss.
		return
	}
	s.ingest(s.ctx, TransportUDP, remote, bytes.Clone(payload))
}

func udpRemote(addr net.Addr) (remoteAddr, bool) {
	u, ok := addr.(*net.UDPAddr)
	if !ok || u == nil {
		return remoteAddr{}, false
	}
	ap := u.AddrPort()
	return remoteAddr{ip: ap.Addr().Unmap(), port: ap.Port()}, true
}

func isClosedConn(err error) bool {
	return errors.Is(err, net.ErrClosed)
}
