package syslogwire

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func FuzzParse(f *testing.F) {
	now := time.Date(2026, 9, 4, 20, 52, 35, 0, time.UTC)
	seeds := [][]byte{
		[]byte(`<34>Sep  4 20:52:35 sut-1 sshd[1234]: Failed password`),
		[]byte(`<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 ID47 [sshd@0 user="alice"] Failed password`),
		[]byte(`<13>1 - - - - - -`),
		[]byte(`<13>1 2026-09-04T20:52:35Z h a - - [ex p="quote=\" slash\\ bracket\]"] msg`),
		[]byte(`Sep  4 20:52:35 sut-1 sshd[1234]: Failed password`),
		[]byte(`<13>1 2026-09-04T20:52:35Z host app - - `),
		append([]byte(`<13>1 2026-09-04T20:52:35Z host app - - `), 0xEF, 0xBB, 0xBF, 'h'),
		[]byte(`<200>Sep  4 20:52:35 sut-1 app: hi`),
		[]byte(``),
		[]byte(`<13>`),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	root := packetsRoot(f)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var g goldenFile
		if err := json.Unmarshal(body, &g); err != nil {
			return nil
		}
		if g.RawBase64 != "" {
			raw, err := base64.StdEncoding.DecodeString(g.RawBase64)
			if err == nil {
				f.Add(raw)
			}
			return nil
		}
		if g.Raw != "" {
			f.Add([]byte(g.Raw))
		}
		return nil
	})
	for _, raw := range testutil.LoadCorpus(f, "syslogwire") {
		f.Add(raw)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		opts := []Options{
			{RFC3164: true, RFC5424: true, BestEffort: true, Now: now},
			{RFC3164: true, RFC5424: true, BestEffort: false, Now: now},
			{RFC3164: true, RFC5424: false, BestEffort: true, Now: now},
			{RFC3164: false, RFC5424: true, BestEffort: true, Now: now},
			{RFC3164: false, RFC5424: false, BestEffort: true, Now: now},
		}
		for _, o := range opts {
			p, warn, err := Parse(data, o)
			if o.BestEffort && err != nil {
				t.Fatalf("bestEffort returned err %v warn %q parsed %+v", err, warn, p)
			}
			if err == nil {
				_ = Serialize(p)
			}
		}
	})
}
