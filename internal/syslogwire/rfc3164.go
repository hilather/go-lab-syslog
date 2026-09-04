package syslogwire

import (
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
)

var rfc3164Months = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March,
	"Apr": time.April, "May": time.May, "Jun": time.June,
	"Jul": time.July, "Aug": time.August, "Sep": time.September,
	"Oct": time.October, "Nov": time.November, "Dec": time.December,
}

func parseRFC3164(data []byte, bestEffort bool, now time.Time) (model.Parsed, string, error) {
	warn := ""
	n, rest, ok := scanPRI(data)
	var pri uint8
	if !ok {
		if !bestEffort {
			return model.Parsed{}, "", unparseable("missing PRI")
		}
		warn = WarnMissingPRI
		pri = defaultPRI
		rest = data
	} else {
		var out bool
		pri, out = priFromScan(n)
		if out {
			if !bestEffort {
				return model.Parsed{}, "", unparseable("PRI out of range")
			}
			warn = WarnUnknownFacility
		}
	}

	p := parsedPRI(pri)
	p.Version = 0

	ts, rest2, tsOK := parseRFC3164Stamp(rest, now)
	if tsOK {
		p.Timestamp = ts
		rest = rest2
	}

	if len(rest) == 0 {
		return p, warn, nil
	}
	host, rest, _ := nextToken(rest)
	p.Hostname = string(host)

	app, proc, content, tagged := parse3164Tag(rest)
	if tagged {
		p.AppName = app
		p.ProcID = proc
		p.Message = string(content)
		return p, warn, nil
	}
	p.Message = string(rest)
	return p, warn, nil
}

func parseRFC3164Stamp(b []byte, now time.Time) (time.Time, []byte, bool) {
	// Mmm[ SP (SP DIGIT / 2DIGIT) SP hh:mm:ss ]
	if len(b) < 15 {
		return time.Time{}, b, false
	}
	mon, ok := rfc3164Months[string(b[:3])]
	if !ok || b[3] != ' ' {
		return time.Time{}, b, false
	}
	i := 4
	if i < len(b) && b[i] == ' ' {
		i++
	}
	day, i, ok := readDigits(b, i, 1, 2)
	if !ok || day < 1 || day > 31 {
		return time.Time{}, b, false
	}
	if i >= len(b) || b[i] != ' ' {
		return time.Time{}, b, false
	}
	i++
	hour, i, ok := readDigits(b, i, 2, 2)
	if !ok || hour > 23 || i >= len(b) || b[i] != ':' {
		return time.Time{}, b, false
	}
	i++
	min, i, ok := readDigits(b, i, 2, 2)
	if !ok || min > 59 || i >= len(b) || b[i] != ':' {
		return time.Time{}, b, false
	}
	i++
	sec, i, ok := readDigits(b, i, 2, 2)
	if !ok || sec > 60 {
		return time.Time{}, b, false
	}
	if i < len(b) && b[i] != ' ' {
		return time.Time{}, b, false
	}
	if i < len(b) && b[i] == ' ' {
		i++
	}

	loc := now.Location()
	if loc == nil {
		loc = time.Local
	}
	ts := time.Date(now.Year(), mon, day, hour, min, sec, 0, loc)
	if ts.Sub(now) > 24*time.Hour {
		ts = time.Date(now.Year()-1, mon, day, hour, min, sec, 0, loc)
	}
	return ts, b[i:], true
}

func readDigits(b []byte, i, minN, maxN int) (int, int, bool) {
	start := i
	n := 0
	for i < len(b) && i-start < maxN && b[i] >= '0' && b[i] <= '9' {
		n = n*10 + int(b[i]-'0')
		i++
	}
	if i-start < minN {
		return 0, start, false
	}
	return n, i, true
}

func parse3164Tag(b []byte) (app, proc string, rest []byte, ok bool) {
	i := 0
	for i < len(b) && isTagChar(b[i]) {
		i++
	}
	if i == 0 {
		return "", "", b, false
	}
	appEnd := i
	if i < len(b) && b[i] == '[' {
		i++
		j := i
		for j < len(b) && b[j] != ']' {
			j++
		}
		if j >= len(b) {
			return "", "", b, false
		}
		proc = string(b[i:j])
		i = j + 1
	}
	if i >= len(b) || b[i] != ':' {
		return "", "", b, false
	}
	i++
	if i < len(b) && b[i] == ' ' {
		i++
	}
	return string(b[:appEnd]), proc, b[i:], true
}

func isTagChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_' || c == '.':
		return true
	default:
		return false
	}
}
