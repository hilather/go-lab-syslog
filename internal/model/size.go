package model

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ByteSize is a byte count encoded with binary units (`64KiB`, `256MiB`).
type ByteSize int64

const (
	Byte ByteSize = 1
	KiB           = 1024 * Byte
	MiB           = 1024 * KiB
	GiB           = 1024 * MiB
	TiB           = 1024 * GiB
)

// ParseByteSize parses a binary byte-size string.
func ParseByteSize(s string) (ByteSize, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty byte size")
	}
	units := []struct {
		suffix string
		mul    int64
	}{
		{"TiB", int64(TiB)},
		{"GiB", int64(GiB)},
		{"MiB", int64(MiB)},
		{"KiB", int64(KiB)},
		{"B", int64(Byte)},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			n := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			v, err := strconv.ParseInt(n, 10, 64)
			if err != nil || v < 0 {
				return 0, fmt.Errorf("invalid byte size %q", s)
			}
			return ByteSize(v * u.mul), nil
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid byte size %q", s)
	}
	return ByteSize(v), nil
}

func (b ByteSize) Int64() int64 { return int64(b) }

func (b ByteSize) String() string {
	v := int64(b)
	switch {
	case v == 0:
		return "0B"
	case v%int64(TiB) == 0:
		return strconv.FormatInt(v/int64(TiB), 10) + "TiB"
	case v%int64(GiB) == 0:
		return strconv.FormatInt(v/int64(GiB), 10) + "GiB"
	case v%int64(MiB) == 0:
		return strconv.FormatInt(v/int64(MiB), 10) + "MiB"
	case v%int64(KiB) == 0:
		return strconv.FormatInt(v/int64(KiB), 10) + "KiB"
	default:
		return strconv.FormatInt(v, 10) + "B"
	}
}

func (b *ByteSize) UnmarshalText(text []byte) error {
	parsed, err := ParseByteSize(string(text))
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *ByteSize) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] != '"' {
		var n int64
		if err := json.Unmarshal(data, &n); err != nil || n < 0 {
			return fmt.Errorf("invalid byte size")
		}
		*b = ByteSize(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("byte size must be a string")
	}
	return b.UnmarshalText([]byte(s))
}

func (b ByteSize) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.String())
}
