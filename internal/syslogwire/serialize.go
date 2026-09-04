package syslogwire

import (
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
)

var rfc3164MonthNames = [...]string{
	"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// Serialize renders p as RFC 5424 when Version is 1, otherwise RFC 3164.
// Used for tests and export, not forwarding.
func Serialize(p model.Parsed) []byte {
	if p.Version == 1 {
		return serializeRFC5424(p)
	}
	return serializeRFC3164(p)
}

func serializeRFC5424(p model.Parsed) []byte {
	var b strings.Builder
	writePRI(&b, p.PRI)
	b.WriteString("1 ")
	b.WriteString(format5424Time(p.Timestamp))
	b.WriteByte(' ')
	writeNIL(&b, p.Hostname)
	b.WriteByte(' ')
	writeNIL(&b, p.AppName)
	b.WriteByte(' ')
	writeNIL(&b, p.ProcID)
	b.WriteByte(' ')
	writeNIL(&b, p.MsgID)
	b.WriteByte(' ')
	if len(p.Structured) == 0 {
		b.WriteByte('-')
	} else {
		for _, el := range p.Structured {
			b.WriteByte('[')
			b.WriteString(el.ID)
			for _, param := range el.Params {
				b.WriteByte(' ')
				b.WriteString(param.Name)
				b.WriteString(`="`)
				writeSDEscaped(&b, param.Value)
				b.WriteByte('"')
			}
			b.WriteByte(']')
		}
	}
	if p.Message != "" {
		b.WriteByte(' ')
		b.WriteString(p.Message)
	}
	return []byte(b.String())
}

func serializeRFC3164(p model.Parsed) []byte {
	var b strings.Builder
	writePRI(&b, p.PRI)
	b.WriteString(format3164Time(p.Timestamp))
	b.WriteByte(' ')
	if p.Hostname != "" {
		b.WriteString(p.Hostname)
	} else {
		b.WriteByte('-')
	}
	if p.AppName != "" {
		b.WriteByte(' ')
		b.WriteString(p.AppName)
		if p.ProcID != "" {
			b.WriteByte('[')
			b.WriteString(p.ProcID)
			b.WriteByte(']')
		}
		b.WriteString(": ")
		b.WriteString(p.Message)
	} else if p.Message != "" {
		b.WriteByte(' ')
		b.WriteString(p.Message)
	}
	return []byte(b.String())
}

func writePRI(b *strings.Builder, pri uint8) {
	b.WriteByte('<')
	b.WriteString(strconv.FormatUint(uint64(pri), 10))
	b.WriteByte('>')
}

func writeNIL(b *strings.Builder, s string) {
	if s == "" {
		b.WriteByte('-')
		return
	}
	b.WriteString(s)
}

func writeSDEscaped(b *strings.Builder, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\', ']':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
}

func format5424Time(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	if t.Nanosecond() == 0 {
		return t.Format("2006-01-02T15:04:05Z07:00")
	}
	// RFC 5424 allows at most 6 fractional digits.
	usec := t.Nanosecond() / 1000
	trimmed := t.Truncate(time.Microsecond)
	if usec%1000 == 0 {
		if usec%1_000_000 == 0 {
			return trimmed.Format("2006-01-02T15:04:05Z07:00")
		}
		return trimmed.Format("2006-01-02T15:04:05.000Z07:00")
	}
	return trimmed.Format("2006-01-02T15:04:05.000000Z07:00")
}

func format3164Time(t time.Time) string {
	if t.IsZero() {
		t = time.Unix(0, 0).UTC()
	}
	mon := rfc3164MonthNames[t.Month()]
	return mon + " " + t.Format("_2 15:04:05")
}
