package syslogframing

import (
	"bufio"
	"errors"
	"io"
)

// Frozen spec.listeners.tcp.framing values (docs/02, docs/04). Abbreviations
// such as "octet" or "newline" are not 1.0 names.
const (
	Auto           = "auto"
	OctetCounting  = "octet-counting"
	NonTransparent = "non-transparent"
)

const defaultMax = 64 * 1024

// ErrFraming is a session-fatal RFC 6587 error (close; do not store a partial).
var ErrFraming = errors.New("tcp framing error")

// ErrOversize is a non-transparent frame larger than maxMessageBytes.
// The trailer has been consumed; the session may continue.
var ErrOversize = errors.New("non-transparent frame exceeds maxMessageBytes")

// Scanner splits a TCP byte stream into SYSLOG-MSG frames (trailers stripped).
type Scanner struct {
	br     *bufio.Reader
	mode   string
	sticky string
	max    int
	seen   bool
}

// NewScanner reads frames from r. mode is auto, octet-counting, or
// non-transparent (empty means auto). maxMessageBytes is the SYSLOG-MSG cap.
func NewScanner(r io.Reader, mode string, maxMessageBytes int) *Scanner {
	if mode == "" {
		mode = Auto
	}
	if maxMessageBytes <= 0 {
		maxMessageBytes = defaultMax
	}
	return &Scanner{
		br:   bufio.NewReader(r),
		mode: mode,
		max:  maxMessageBytes,
	}
}

// Mode is the sticky framing in force. Empty until auto has seen the first bytes.
func (s *Scanner) Mode() string {
	if s == nil {
		return ""
	}
	if s.sticky != "" {
		return s.sticky
	}
	return s.mode
}

// Next returns the next SYSLOG-MSG. Trailers (LF, NUL, optional CR before LF)
// are not part of the frame. io.EOF ends the stream. ErrOversize drops one
// non-transparent frame; Next may be called again. ErrFraming is terminal.
func (s *Scanner) Next() ([]byte, error) {
	if s.mode != Auto && s.mode != OctetCounting && s.mode != NonTransparent {
		return nil, ErrFraming
	}
	if err := s.resolve(); err != nil {
		return nil, err
	}
	if s.mode == Auto && s.seen {
		if err := s.agree(); err != nil {
			return nil, err
		}
	}
	var (
		frame []byte
		err   error
	)
	if s.sticky == OctetCounting {
		frame, err = s.nextOctet()
	} else {
		frame, err = s.nextNonTransparent()
	}
	// Sticky auto is decided on the first bytes, including an oversize
	// first frame, so later DIGIT+ SP can disagree (D11).
	s.seen = true
	return frame, err
}

func (s *Scanner) resolve() error {
	if s.sticky != "" {
		return nil
	}
	if s.mode != Auto {
		s.sticky = s.mode
		return nil
	}
	ok, err := s.peekOctet()
	if err != nil {
		return err
	}
	if ok {
		s.sticky = OctetCounting
	} else {
		s.sticky = NonTransparent
	}
	return nil
}

func (s *Scanner) agree() error {
	ok, err := s.peekOctet()
	if err != nil {
		return err
	}
	if s.sticky == OctetCounting && !ok {
		return ErrFraming
	}
	if s.sticky == NonTransparent && ok {
		return ErrFraming
	}
	return nil
}

// peekOctet reports whether the next bytes are DIGIT+ SP (RFC 6587 octet-counting).
func (s *Scanner) peekOctet() (bool, error) {
	const maxDigits = 20
	for n := 1; n <= maxDigits; n++ {
		buf, err := s.br.Peek(n)
		if len(buf) < n {
			if len(buf) == 0 && err != nil {
				return false, err
			}
			if errors.Is(err, io.EOF) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return false, io.ErrUnexpectedEOF
		}
		last := buf[n-1]
		if last >= '0' && last <= '9' {
			continue
		}
		if n == 1 {
			return false, nil
		}
		return last == ' ', nil
	}
	return true, nil
}

func (s *Scanner) nextOctet() ([]byte, error) {
	first, err := s.br.ReadByte()
	if err != nil {
		return nil, err
	}
	if first < '0' || first > '9' {
		return nil, ErrFraming
	}
	leadingZero := first == '0'
	n := int(first - '0')
	for {
		b, err := s.br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, ErrFraming
			}
			return nil, err
		}
		if b >= '0' && b <= '9' {
			if leadingZero {
				return nil, ErrFraming
			}
			d := int(b - '0')
			if n > (s.max-d)/10 || n*10+d > s.max {
				return nil, ErrFraming
			}
			n = n*10 + d
			continue
		}
		if b != ' ' {
			return nil, ErrFraming
		}
		break
	}
	if n == 0 || n > s.max {
		return nil, ErrFraming
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(s.br, buf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrFraming
		}
		return nil, err
	}
	return buf, nil
}

func (s *Scanner) nextNonTransparent() ([]byte, error) {
	var buf []byte
	oversize := false
	for {
		b, err := s.br.ReadByte()
		if err != nil {
			if !oversize && len(buf) == 0 && errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, ErrFraming
			}
			return nil, err
		}
		if b == '\n' || b == 0 {
			if b == '\n' && len(buf) > 0 && buf[len(buf)-1] == '\r' {
				buf = buf[:len(buf)-1]
			}
			// Size is SYSLOG-MSG after trailer strip (CRLF CR is not content).
			if oversize || len(buf) > s.max {
				return nil, ErrOversize
			}
			return buf, nil
		}
		if oversize {
			continue
		}
		if len(buf) == s.max && b == '\r' {
			// Pending CR may still be a CRLF trailer, not a max+1 byte.
			buf = append(buf, b)
			continue
		}
		if len(buf) >= s.max {
			oversize = true
			buf = nil
			continue
		}
		buf = append(buf, b)
	}
}
