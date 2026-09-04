package syslogwire

import (
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

// Frozen parseWarning tokens (docs/02). Several tokens are comma-separated.
const (
	WarnMissingPRI      = "missing_pri"
	WarnUTF8BOM         = "utf8_bom"
	WarnNoParser        = "no_parser"
	WarnRFC5424Disabled = "rfc5424_disabled"
	WarnUnknownFacility = "unknown_facility"
)

// Options selects parsers. Now is the process clock used to infer the
// RFC 3164 year; a zero Now uses time.Now.
type Options struct {
	RFC3164    bool
	RFC5424    bool
	BestEffort bool
	Now        time.Time
}

// DefaultOptions enables both parsers and best-effort (product YAML defaults).
func DefaultOptions() Options {
	return Options{RFC3164: true, RFC5424: true, BestEffort: true}
}

// OptionsFromParse maps spec.syslog.parse. Nil flags keep product defaults.
func OptionsFromParse(p model.SyslogParse) Options {
	o := DefaultOptions()
	if p.RFC3164 != nil {
		o.RFC3164 = *p.RFC3164
	}
	if p.RFC5424 != nil {
		o.RFC5424 = *p.RFC5424
	}
	if p.BestEffort != nil {
		o.BestEffort = *p.BestEffort
	}
	return o
}

// Parse SYSLOG-MSG bytes into model.Parsed. A drop returns
// domainerr.Unparseable; best-effort never drops.
func Parse(data []byte, opts Options) (model.Parsed, string, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	if looksLikeRFC5424(data) {
		if opts.RFC5424 {
			return parseRFC5424(data, opts.BestEffort, now)
		}
		if opts.BestEffort {
			p, w := recoverPrefix(data)
			p.Version = 1
			return p, joinWarn(w, WarnRFC5424Disabled), nil
		}
		return model.Parsed{}, "", unparseable("rfc5424 disabled")
	}
	if opts.RFC3164 {
		return parseRFC3164(data, opts.BestEffort, now)
	}
	if opts.BestEffort {
		p, w := recoverPrefix(data)
		return p, joinWarn(w, WarnNoParser), nil
	}
	return model.Parsed{}, "", unparseable("no parser")
}

func looksLikeRFC5424(data []byte) bool {
	n, rest, ok := scanPRI(data)
	if !ok || n > 999 {
		return false
	}
	return len(rest) >= 2 && rest[0] == '1' && rest[1] == ' '
}

func recoverPrefix(data []byte) (model.Parsed, string) {
	n, _, ok := scanPRI(data)
	if !ok {
		return model.Parsed{}, ""
	}
	pri, out := priFromScan(n)
	p := parsedPRI(pri)
	if out {
		return p, WarnUnknownFacility
	}
	return p, ""
}

func joinWarn(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "," + b
}

func unparseable(detail string) error {
	return domainerr.New(domainerr.Unparseable, detail)
}

func nextToken(b []byte) (token []byte, rest []byte, ok bool) {
	if len(b) == 0 {
		return nil, b, false
	}
	i := 0
	for i < len(b) && b[i] != ' ' {
		i++
	}
	token = b[:i]
	if i == len(b) {
		return token, nil, true
	}
	return token, b[i+1:], true
}

func isNIL(b []byte) bool {
	return len(b) == 1 && b[0] == '-'
}
