package observability

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOpenMetricsParsesAndHasRequiredSeries(t *testing.T) {
	var buf bytes.Buffer
	err := WriteOpenMetrics(&buf, Snapshot{
		ReceivedByTransport: map[string]uint64{TransportUDP: 3},
		StoredBy:            map[string]uint64{StoredKey(TransportUDP, ProtocolRFC3164): 2},
		DroppedByReason:     map[string]uint64{"oversize": 1, "empty": 4},
		AdmissionByReason:   map[string]uint64{"admission_cidr": 1},
		UDPOversize:         1,
		StoreMessages:       2,
		StoreBytes:          512,
		StoreGeneration:     9,
		StoreEvicted:        1,
		StoreRejected:       0,
		Waiters:             0,
		ApplyByResult:       map[string]uint64{"ok": 1},
		HTTPRequests:        []HTTPSample{{Code: 200, Route: "/v1/metrics", Value: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if !strings.HasSuffix(strings.TrimSpace(text), "# EOF") {
		t.Fatal("missing EOF")
	}
	samples, err := ParseOpenMetrics(text)
	if err != nil {
		t.Fatal(err)
	}
	by := ByName(samples)
	for _, name := range RequiredNames() {
		if _, ok := by[name]; !ok {
			t.Errorf("missing required series %s", name)
		}
	}
	if _, ok := by["labsyslog_store_rejected_total"]; !ok {
		t.Error("missing labsyslog_store_rejected_total")
	}
	if got := sampleValue(by["labsyslog_messages_received_total"], map[string]string{"transport": "udp"}); got != 3 {
		t.Fatalf("received udp = %v", got)
	}
	if got := sampleValue(by["labsyslog_messages_received_total"], map[string]string{"transport": "tcp"}); got != 0 {
		t.Fatalf("received tcp zero-fill = %v", got)
	}
	if got := sampleValue(by["labsyslog_messages_stored_total"], map[string]string{"transport": "udp", "protocol": "rfc3164"}); got != 2 {
		t.Fatalf("stored = %v", got)
	}
	if strings.Contains(text, "client_ip") || strings.Contains(text, "remoteIP") {
		t.Fatal("client-IP labels are forbidden")
	}
}

func TestParseOpenMetricsRejectsPrometheusTextWithoutEOF(t *testing.T) {
	_, err := ParseOpenMetrics("# TYPE x counter\nx_total 1\n")
	if err == nil {
		t.Fatal("expected missing EOF")
	}
}

func TestCatalogCoversRequiredNames(t *testing.T) {
	got := map[string]bool{}
	for _, s := range Catalog() {
		got[s.Name] = true
		if s.Type != TypeCounter && s.Type != TypeGauge {
			t.Errorf("%s type %s", s.Name, s.Type)
		}
		if s.Help == "" {
			t.Errorf("%s missing help", s.Name)
		}
	}
	for _, name := range RequiredNames() {
		if !got[name] {
			t.Errorf("catalog missing %s", name)
		}
	}
}

func TestNoPrometheusImportAST(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if p == "github.com/prometheus" || strings.HasPrefix(p, "github.com/prometheus/") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %s", rel, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mod, []byte("github.com/prometheus")) {
		t.Fatal("go.mod must not require github.com/prometheus/*")
	}
}

func sampleValue(samples []Sample, want map[string]string) float64 {
	for _, s := range samples {
		if labelsEqual(s.Labels, want) {
			return s.Value
		}
	}
	return -1
}

func labelsEqual(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
