package syslogwire

import (
	"bytes"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hilather/go-lab-syslog/internal/model"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func parseRFC5424(data []byte, bestEffort bool, _ time.Time) (model.Parsed, string, error) {
	warn := ""
	n, rest, ok := scanPRI(data)
	if !ok {
		if !bestEffort {
			return model.Parsed{}, "", unparseable("missing PRI")
		}
		return parsedPRI(defaultPRI), WarnMissingPRI, nil
	}
	pri, out := priFromScan(n)
	p := parsedPRI(pri)
	p.Version = 1
	if out {
		warn = WarnUnknownFacility
	}
	if len(rest) < 2 || rest[0] != '1' || rest[1] != ' ' {
		if !bestEffort {
			return p, warn, unparseable("missing version")
		}
		p.Message = string(rest)
		return p, warn, nil
	}
	rest = rest[2:]

	fail := func(detail string) (model.Parsed, string, error) {
		if bestEffort {
			p.Message = string(rest)
			return p, warn, nil
		}
		return p, warn, unparseable(detail)
	}

	tsTok, rest, ok := nextToken(rest)
	if !ok {
		return fail("missing timestamp")
	}
	ts, tsOK := parse5424Timestamp(tsTok)
	if !tsOK && !isNIL(tsTok) {
		if !bestEffort {
			return p, warn, unparseable("invalid timestamp")
		}
	} else {
		p.Timestamp = ts
	}

	host, rest, ok := nextToken(rest)
	if !ok {
		return fail("missing hostname")
	}
	if !isNIL(host) {
		p.Hostname = string(host)
	}

	app, rest, ok := nextToken(rest)
	if !ok {
		return fail("missing app-name")
	}
	if !isNIL(app) {
		p.AppName = string(app)
	}

	proc, rest, ok := nextToken(rest)
	if !ok {
		return fail("missing procid")
	}
	if !isNIL(proc) {
		p.ProcID = string(proc)
	}

	msgid, rest, ok := nextToken(rest)
	if !ok {
		return fail("missing msgid")
	}
	if !isNIL(msgid) {
		p.MsgID = string(msgid)
	}

	if len(rest) == 0 {
		if !bestEffort {
			return p, warn, unparseable("missing structured-data")
		}
		return p, warn, nil
	}

	elems, rest2, err := parseStructuredData(rest, bestEffort)
	if err != nil {
		if !bestEffort {
			return p, warn, err
		}
		p.Structured = elems
		p.Message = string(rest2)
		return p, warn, nil
	}
	p.Structured = elems
	rest = rest2

	if len(rest) == 0 {
		return p, warn, nil
	}
	if rest[0] == ' ' {
		rest = rest[1:]
	}
	if bytes.HasPrefix(rest, utf8BOM) {
		rest = rest[len(utf8BOM):]
		warn = joinWarn(warn, WarnUTF8BOM)
	}
	p.Message = string(rest)
	return p, warn, nil
}

func parse5424Timestamp(tok []byte) (time.Time, bool) {
	if isNIL(tok) {
		return time.Time{}, true
	}
	s := string(tok)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func parseStructuredData(b []byte, bestEffort bool) ([]model.SDElement, []byte, error) {
	if isNIL(b) || (len(b) >= 1 && b[0] == '-' && (len(b) == 1 || b[1] == ' ')) {
		if len(b) > 1 && b[0] == '-' {
			return nil, b[1:], nil
		}
		return nil, nil, nil
	}
	if len(b) == 0 || b[0] != '[' {
		if bestEffort {
			return nil, b, nil
		}
		return nil, b, unparseable("invalid structured-data")
	}
	var elems []model.SDElement
	for len(b) > 0 && b[0] == '[' {
		el, rest, err := parseSDElement(b)
		if err != nil {
			return elems, b, err
		}
		elems = append(elems, el)
		b = rest
	}
	return elems, b, nil
}

func parseSDElement(b []byte) (model.SDElement, []byte, error) {
	var el model.SDElement
	if len(b) < 2 || b[0] != '[' {
		return el, b, unparseable("invalid SD-ELEMENT")
	}
	i := 1
	start := i
	for i < len(b) && b[i] != ' ' && b[i] != ']' {
		if b[i] == '=' || b[i] == '"' {
			return el, b, unparseable("invalid SD-ID")
		}
		i++
	}
	if i == start {
		return el, b, unparseable("empty SD-ID")
	}
	el.ID = string(b[start:i])
	for i < len(b) && b[i] == ' ' {
		i++
		ns := i
		for i < len(b) && b[i] != '=' && b[i] != ' ' && b[i] != ']' {
			i++
		}
		if ns == i || i >= len(b) || b[i] != '=' {
			return el, b, unparseable("invalid SD-PARAM")
		}
		name := string(b[ns:i])
		i++
		if i >= len(b) || b[i] != '"' {
			return el, b, unparseable("invalid SD-PARAM value")
		}
		i++
		val, ni, err := parseSDValue(b, i)
		if err != nil {
			return el, b, err
		}
		i = ni
		el.Params = append(el.Params, model.SDParam{Name: name, Value: val})
	}
	if i >= len(b) || b[i] != ']' {
		return el, b, unparseable("unclosed SD-ELEMENT")
	}
	return el, b[i+1:], nil
}

func parseSDValue(b []byte, i int) (string, int, error) {
	var buf strings.Builder
	for i < len(b) {
		c := b[i]
		if c == '\\' {
			if i+1 >= len(b) {
				return "", i, unparseable("truncated SD escape")
			}
			n := b[i+1]
			if n != '"' && n != '\\' && n != ']' {
				return "", i, unparseable("invalid SD escape")
			}
			buf.WriteByte(n)
			i += 2
			continue
		}
		if c == '"' {
			return buf.String(), i + 1, nil
		}
		if c == ']' {
			return "", i, unparseable("unescaped ] in SD-PARAM")
		}
		if c < utf8.RuneSelf {
			buf.WriteByte(c)
			i++
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			buf.WriteByte(c)
			i++
			continue
		}
		buf.WriteRune(r)
		i += size
	}
	return "", i, unparseable("unterminated SD-PARAM")
}
