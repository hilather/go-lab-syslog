package testutil

import "testing"

func TestFuzzCorpusNonEmpty(t *testing.T) {
	wire := LoadCorpus(t, "syslogwire")
	framing := LoadCorpus(t, "syslogframing")
	if len(wire) == 0 || len(framing) == 0 {
		t.Fatalf("corpus sizes wire=%d framing=%d", len(wire), len(framing))
	}
}
