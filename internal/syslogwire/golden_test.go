package syslogwire

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

type goldenFile struct {
	Comment   string         `json:"comment"`
	Raw       string         `json:"raw"`
	RawBase64 string         `json:"rawBase64"`
	Now       string         `json:"now"`
	Options   *goldenOptions `json:"options"`
	Warning   string         `json:"warning"`
	Error     string         `json:"error"`
	Parsed    goldenParsed   `json:"parsed"`
}

type goldenOptions struct {
	RFC3164    bool `json:"rfc3164"`
	RFC5424    bool `json:"rfc5424"`
	BestEffort bool `json:"bestEffort"`
}

type goldenParsed struct {
	PRI        uint8             `json:"pri"`
	Facility   uint8             `json:"facility"`
	Severity   uint8             `json:"severity"`
	Version    uint8             `json:"version"`
	Timestamp  string            `json:"timestamp,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	AppName    string            `json:"appName,omitempty"`
	ProcID     string            `json:"procID,omitempty"`
	MsgID      string            `json:"msgID,omitempty"`
	Structured []goldenSDElement `json:"structured,omitempty"`
	Message    string            `json:"message,omitempty"`
}

type goldenSDElement struct {
	ID     string          `json:"id"`
	Params []goldenSDParam `json:"params,omitempty"`
}

type goldenSDParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func TestGoldenPackets(t *testing.T) {
	root := packetsRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden packets under testdata/packets")
	}
	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			runGolden(t, path)
		})
	}
}

func runGolden(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw := []byte(g.Raw)
	if g.RawBase64 != "" {
		raw, err = base64.StdEncoding.DecodeString(g.RawBase64)
		if err != nil {
			t.Fatalf("rawBase64: %v", err)
		}
	}
	opts := DefaultOptions()
	if g.Options != nil {
		opts = Options{
			RFC3164:    g.Options.RFC3164,
			RFC5424:    g.Options.RFC5424,
			BestEffort: g.Options.BestEffort,
		}
	}
	now := time.Date(2026, 9, 4, 20, 52, 35, 0, time.UTC)
	if g.Now != "" {
		now, err = time.Parse(time.RFC3339Nano, g.Now)
		if err != nil {
			t.Fatalf("now: %v", err)
		}
	}
	opts.Now = now

	got, warn, err := Parse(raw, opts)
	if g.Error != "" {
		if err == nil {
			t.Fatalf("error = nil, want %s", g.Error)
		}
		if !domainerr.Is(err, domainerr.Code(g.Error)) {
			t.Fatalf("error = %v, want code %s", err, g.Error)
		}
		if warn != g.Warning {
			t.Fatalf("warning = %q, want %q", warn, g.Warning)
		}
		return
	}
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if warn != g.Warning {
		t.Fatalf("warning = %q, want %q", warn, g.Warning)
	}
	want := fromGoldenParsed(t, g.Parsed)
	assertParsedEqual(t, got, want)
}

func fromGoldenParsed(t *testing.T, g goldenParsed) model.Parsed {
	t.Helper()
	p := model.Parsed{
		PRI:      g.PRI,
		Facility: g.Facility,
		Severity: g.Severity,
		Version:  g.Version,
		Hostname: g.Hostname,
		AppName:  g.AppName,
		ProcID:   g.ProcID,
		MsgID:    g.MsgID,
		Message:  g.Message,
	}
	if g.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339Nano, g.Timestamp)
		if err != nil {
			t.Fatalf("parsed.timestamp: %v", err)
		}
		p.Timestamp = ts
	}
	for _, el := range g.Structured {
		out := model.SDElement{ID: el.ID}
		for _, param := range el.Params {
			out.Params = append(out.Params, model.SDParam{Name: param.Name, Value: param.Value})
		}
		p.Structured = append(p.Structured, out)
	}
	return p
}

func assertParsedEqual(t *testing.T, got, want model.Parsed) {
	t.Helper()
	if got.PRI != want.PRI || got.Facility != want.Facility || got.Severity != want.Severity || got.Version != want.Version {
		t.Errorf("pri/fac/sev/ver = %d/%d/%d/%d, want %d/%d/%d/%d",
			got.PRI, got.Facility, got.Severity, got.Version,
			want.PRI, want.Facility, want.Severity, want.Version)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.Timestamp.Location().String() != want.Timestamp.Location().String() {
		t.Errorf("timestamp = %s (%s), want %s (%s)",
			got.Timestamp.Format(time.RFC3339Nano), got.Timestamp.Location(),
			want.Timestamp.Format(time.RFC3339Nano), want.Timestamp.Location())
	}
	if got.Hostname != want.Hostname || got.AppName != want.AppName || got.ProcID != want.ProcID || got.MsgID != want.MsgID {
		t.Errorf("host/app/proc/msgid = %q %q %q %q, want %q %q %q %q",
			got.Hostname, got.AppName, got.ProcID, got.MsgID,
			want.Hostname, want.AppName, want.ProcID, want.MsgID)
	}
	if got.Message != want.Message {
		t.Errorf("message = %q, want %q", got.Message, want.Message)
	}
	if len(got.Structured) != len(want.Structured) {
		t.Errorf("structured len = %d, want %d (%+v vs %+v)", len(got.Structured), len(want.Structured), got.Structured, want.Structured)
		return
	}
	for i := range want.Structured {
		if got.Structured[i].ID != want.Structured[i].ID {
			t.Errorf("structured[%d].id = %q, want %q", i, got.Structured[i].ID, want.Structured[i].ID)
		}
		if len(got.Structured[i].Params) != len(want.Structured[i].Params) {
			t.Errorf("structured[%d].params = %+v, want %+v", i, got.Structured[i].Params, want.Structured[i].Params)
			continue
		}
		for j := range want.Structured[i].Params {
			gp := got.Structured[i].Params[j]
			wp := want.Structured[i].Params[j]
			if gp.Name != wp.Name || gp.Value != wp.Value {
				t.Errorf("structured[%d].params[%d] = {%q %q}, want {%q %q}", i, j, gp.Name, gp.Value, wp.Name, wp.Value)
			}
		}
	}
}

func packetsRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "testdata", "packets")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("testdata/packets not found")
		}
		dir = parent
	}
}
