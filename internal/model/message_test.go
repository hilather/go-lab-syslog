package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessageTruncatedJSONRoundTrip(t *testing.T) {
	m := Message{}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"truncated":false`) {
		t.Fatalf("truncated missing or not false: %s", b)
	}
	var back Message
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Truncated {
		t.Fatal("truncated round-tripped as true; 1.0 requires false")
	}
}

func TestByteSizeRoundTrip(t *testing.T) {
	for _, s := range []string{"64KiB", "256MiB", "1MiB", "1GiB"} {
		n, err := ParseByteSize(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if n.String() != s {
			t.Fatalf("%s string = %q", s, n.String())
		}
		text, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		var back ByteSize
		if err := json.Unmarshal(text, &back); err != nil {
			t.Fatal(err)
		}
		if back != n {
			t.Fatalf("%s json round-trip %d != %d", s, back, n)
		}
	}
}

func TestDurationCompact(t *testing.T) {
	d, err := ParseDuration("2m")
	if err != nil {
		t.Fatal(err)
	}
	if d.String() != "2m" {
		t.Fatalf("got %q", d.String())
	}
	s, err := ParseDuration("60s")
	if err != nil {
		t.Fatal(err)
	}
	if s.String() != "60s" {
		t.Fatalf("60s compact = %q", s.String())
	}
}
