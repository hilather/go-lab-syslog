package observability

import (
	"fmt"
	"strconv"
	"strings"
)

// Sample is one parsed OpenMetrics data point.
type Sample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// ParseOpenMetrics is a small test/helper parser. It requires # EOF and TYPE lines.
func ParseOpenMetrics(text string) ([]Sample, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasSuffix(strings.TrimSpace(text), "# EOF") {
		return nil, fmt.Errorf("missing # EOF terminator")
	}
	var (
		samples []Sample
		typed   = map[string]bool{}
	)
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "# EOF" {
			continue
		}
		if strings.HasPrefix(line, "# TYPE ") {
			rest := strings.TrimPrefix(line, "# TYPE ")
			fields := strings.Fields(rest)
			if len(fields) < 2 {
				return nil, fmt.Errorf("line %d: malformed TYPE", i+1)
			}
			typed[fields[0]] = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		s, err := parseSample(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		samples = append(samples, s)
	}
	if len(typed) == 0 {
		return nil, fmt.Errorf("no # TYPE lines")
	}
	return samples, nil
}

func parseSample(line string) (Sample, error) {
	name, rest, ok := strings.Cut(line, "{")
	var labels map[string]string
	if ok {
		name = strings.TrimSpace(name)
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			return Sample{}, fmt.Errorf("unclosed labels")
		}
		var err error
		labels, err = parseLabels(rest[:end])
		if err != nil {
			return Sample{}, err
		}
		rest = strings.TrimSpace(rest[end+1:])
	} else {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return Sample{}, fmt.Errorf("missing value")
		}
		name = fields[0]
		rest = strings.Join(fields[1:], " ")
	}
	valStr := strings.Fields(rest)
	if len(valStr) < 1 {
		return Sample{}, fmt.Errorf("missing value")
	}
	v, err := strconv.ParseFloat(valStr[0], 64)
	if err != nil {
		return Sample{}, fmt.Errorf("value: %w", err)
	}
	if name == "" {
		return Sample{}, fmt.Errorf("empty metric name")
	}
	return Sample{Name: name, Labels: labels, Value: v}, nil
}

func parseLabels(s string) (map[string]string, error) {
	out := map[string]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out, nil
	}
	for len(s) > 0 {
		s = strings.TrimLeft(s, ", ")
		if s == "" {
			break
		}
		eq := strings.IndexByte(s, '=')
		if eq < 1 {
			return nil, fmt.Errorf("malformed label")
		}
		key := strings.TrimSpace(s[:eq])
		s = strings.TrimSpace(s[eq+1:])
		if !strings.HasPrefix(s, `"`) {
			return nil, fmt.Errorf("label %s is not quoted", key)
		}
		s = s[1:]
		var val strings.Builder
		escaped := false
		closed := false
		for i := 0; i < len(s); i++ {
			c := s[i]
			if escaped {
				switch c {
				case 'n':
					val.WriteByte('\n')
				case '\\', '"':
					val.WriteByte(c)
				default:
					val.WriteByte(c)
				}
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				s = s[i+1:]
				closed = true
				break
			}
			val.WriteByte(c)
		}
		if !closed {
			return nil, fmt.Errorf("unclosed label %s", key)
		}
		out[key] = val.String()
	}
	return out, nil
}

// ByName groups samples by metric name.
func ByName(samples []Sample) map[string][]Sample {
	out := map[string][]Sample{}
	for _, s := range samples {
		out[s.Name] = append(out[s.Name], s)
	}
	return out
}
