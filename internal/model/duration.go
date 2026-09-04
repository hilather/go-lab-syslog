package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration encoded as a Go duration string (`2m`, `60s`).
type Duration time.Duration

// ParseDuration parses a Go duration string.
func ParseDuration(s string) (Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return Duration(d), nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func (d Duration) String() string { return formatDuration(time.Duration(d)) }

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("duration must be a string")
	}
	return d.UnmarshalText([]byte(s))
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	neg := d < 0
	if neg {
		d = -d
	}
	var s string
	switch {
	case d%time.Hour == 0 && d >= time.Hour:
		s = fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0 && d >= 2*time.Minute:
		s = fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0 && d >= time.Second:
		s = fmt.Sprintf("%ds", d/time.Second)
	case d%time.Millisecond == 0 && d >= time.Millisecond:
		s = fmt.Sprintf("%dms", d/time.Millisecond)
	case d%time.Microsecond == 0 && d >= time.Microsecond:
		s = fmt.Sprintf("%dus", d/time.Microsecond)
	default:
		s = d.String()
	}
	if neg {
		return "-" + s
	}
	return s
}
