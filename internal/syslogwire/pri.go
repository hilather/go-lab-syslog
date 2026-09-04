package syslogwire

import "github.com/hilather/go-lab-syslog/internal/model"

// RFC 5427 keywords from docs/02. Index is the numeric facility/severity.
var facilityKeywords = [...]string{
	"kern", "user", "mail", "daemon", "auth", "syslog", "lpr", "news",
	"uucp", "cron", "authpriv", "ftp", "ntp", "audit", "console", "cron2",
	"local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7",
}

var severityKeywords = [...]string{
	"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug",
}

const (
	maxPRI = 191
	// user.notice — injected when PRI is missing and bestEffort is set.
	defaultPRI = 13
)

// FacilityKeyword returns the RFC 5427 name for facility 0–23.
func FacilityKeyword(facility uint8) string {
	if int(facility) >= len(facilityKeywords) {
		return ""
	}
	return facilityKeywords[facility]
}

// SeverityKeyword returns the RFC 5427 name for severity 0–7.
func SeverityKeyword(severity uint8) string {
	if int(severity) >= len(severityKeywords) {
		return ""
	}
	return severityKeywords[severity]
}

// LookupFacility maps a keyword or decimal number to 0–23.
func LookupFacility(s string) (uint8, bool) {
	for i, k := range facilityKeywords {
		if k == s {
			return uint8(i), true
		}
	}
	n, ok := atoiBounded(s, 0, 23)
	if !ok {
		return 0, false
	}
	return uint8(n), true
}

// LookupSeverity maps a keyword or decimal number to 0–7.
func LookupSeverity(s string) (uint8, bool) {
	for i, k := range severityKeywords {
		if k == s {
			return uint8(i), true
		}
	}
	n, ok := atoiBounded(s, 0, 7)
	if !ok {
		return 0, false
	}
	return uint8(n), true
}

func atoiBounded(s string, min, max int) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > max {
			return 0, false
		}
	}
	if n < min {
		return 0, false
	}
	return n, true
}

func parsedPRI(pri uint8) model.Parsed {
	return model.Parsed{
		PRI:      pri,
		Facility: pri / 8,
		Severity: pri % 8,
	}
}

// scanPRI reads `<` 1–3 digits `>`. n may exceed 191; ok is false if the
// angle-bracket form is missing.
func scanPRI(data []byte) (n int, rest []byte, ok bool) {
	if len(data) < 3 || data[0] != '<' {
		return 0, data, false
	}
	i := 1
	digits := 0
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		n = n*10 + int(data[i]-'0')
		digits++
		i++
		if digits > 3 {
			return 0, data, false
		}
	}
	if digits == 0 || i >= len(data) || data[i] != '>' {
		return 0, data, false
	}
	return n, data[i+1:], true
}

func priFromScan(n int) (pri uint8, outOfRange bool) {
	if n > 255 {
		return 255, true
	}
	p := uint8(n)
	return p, n > maxPRI || p/8 > 23
}
