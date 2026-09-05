package app

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
	"github.com/hilather/go-lab-syslog/internal/syslogtest"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func TestApplyStaleRevision(t *testing.T) {
	svc := newTestService(t, "")
	_, err := svc.Apply(testutil.Context(t), ApplyRequest{
		ExpectedRevision: "sha256:" + strings.Repeat("0", 64),
		Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "debug"}},
		IdempotencyKey:   "stale",
	})
	if !domainerr.Is(err, domainerr.RevisionMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestIdempotentApply(t *testing.T) {
	svc := newTestService(t, "")
	rev := svc.State(testutil.Context(t)).Revision
	req := ApplyRequest{
		ExpectedRevision: rev,
		Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "debug"}},
		IdempotencyKey:   "dup",
	}
	first, err := svc.Apply(testutil.Context(t), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision == rev {
		t.Fatal("revision did not change")
	}
	second, err := svc.Apply(testutil.Context(t), req)
	if err != nil {
		t.Fatal(err)
	}
	if !second.IdempotentReplay {
		t.Fatal("expected idempotent replay")
	}
	if second.Revision != first.Revision {
		t.Fatalf("replay revision %s != %s", second.Revision, first.Revision)
	}
	if svc.State(testutil.Context(t)).Document.Spec.Observability.LogLevel != "debug" {
		t.Fatal("logLevel changed on replay")
	}
}

func TestApplyImmutableFields(t *testing.T) {
	svc := newTestService(t, "")
	ctx := testutil.Context(t)
	cases := []struct {
		name string
		mut  func(*model.Document)
		want string
	}{
		{"udp.address", func(d *model.Document) { d.Spec.Listeners.UDP.Address = "127.0.0.1:9999" }, "listeners.udp.address"},
		{"bodyLimit", func(d *model.Document) { d.Spec.Management.BodyLimit = 2 * model.MiB }, "bodyLimit"},
		{"hostname", func(d *model.Document) { d.Spec.Syslog.Hostname = "other.lab" }, "hostname"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canon, err := svc.Export(ctx)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := config.DecodeYAML(canon)
			if err != nil {
				t.Fatal(err)
			}
			tc.mut(doc)
			_, err = svc.Apply(ctx, ApplyRequest{
				ExpectedRevision: svc.State(ctx).Revision,
				Candidate:        doc,
				IdempotencyKey:   "imm-" + tc.name,
			})
			if !domainerr.Is(err, domainerr.ImmutableField) {
				t.Fatalf("got %v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("detail %v want %s", err, tc.want)
			}
		})
	}
}

func TestApplyReplaceSyslogParseAndObservability(t *testing.T) {
	svc := newTestService(t, "")
	ctx := testutil.Context(t)
	rev := svc.State(ctx).Revision
	max := 8 * model.KiB
	tru := true
	_, err := svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: rev,
		Operations: []Operation{{
			Type:            OpReplaceSyslogParse,
			Parse:           &model.SyslogParse{RFC3164: &tru, RFC5424: &tru, BestEffort: &tru},
			MaxMessageBytes: &max,
		}},
		IdempotencyKey: "parse",
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Snapshot().Document.Spec.Syslog.MaxMessageBytes != max {
		t.Fatalf("maxMessageBytes = %s", svc.Snapshot().Document.Spec.Syslog.MaxMessageBytes)
	}
	rev = svc.State(ctx).Revision
	_, err = svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: rev,
		Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "warn"}},
		IdempotencyKey:   "obs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Snapshot().Document.Spec.Observability.LogLevel != "warn" {
		t.Fatal("logLevel not updated")
	}
	if svc.Snapshot().Document.Spec.Observability.Metrics.PublicPath {
		t.Fatal("publicPath must stay reset-only")
	}
}

func TestResetWipesStoreAndRestoresFilters(t *testing.T) {
	svc := newTestService(t, `
  filters:
    - name: boot-keep
      action:
        mode: capture
`)
	ctx := testutil.Context(t)
	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	rev := svc.State(ctx).Revision
	_, err := svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: rev,
		Operations: []Operation{{
			Type: OpReplaceFilters,
			Filters: []model.Filter{{
				Name:   "live-drop",
				Action: model.FilterAction{Mode: "drop-silent"},
			}},
		}},
		IdempotencyKey: "filters",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(svc.Snapshot().Document.Spec.Filters); n != 1 || svc.Snapshot().Document.Spec.Filters[0].Name != "live-drop" {
		t.Fatalf("live filters = %+v", svc.Snapshot().Document.Spec.Filters)
	}

	path := svc.cfg.BootstrapPath
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("reset wrote the bootstrap file")
	}
	if svc.Messages().Stats().Messages != 0 {
		t.Fatal("reset did not wipe store")
	}
	filters := svc.Snapshot().Document.Spec.Filters
	if len(filters) != 1 || filters[0].Name != "boot-keep" {
		t.Fatalf("filters after reset = %+v", filters)
	}
	if svc.State(ctx).Drifted {
		t.Fatal("reset with no listen overlays should clear drifted")
	}
}

func TestApplyReplaceFiltersAndBehaviorLive(t *testing.T) {
	svc := newTestService(t, "")
	ctx := testutil.Context(t)
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })

	addr := svc.UDPAddr()
	if addr == nil {
		t.Fatal("udp not bound")
	}
	sendUDP(t, addr.String(), []byte("<14>hello-live"))
	syslogtest.Wait(t, svc.Messages(), store.ListFilter{Transport: store.TransportUDP}, 0)
	if n := svc.Messages().Stats().Messages; n != 1 {
		t.Fatalf("stored %d", n)
	}

	rev := svc.State(ctx).Revision
	_, err := svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: rev,
		Operations: []Operation{{
			Type: OpReplaceFilters,
			Filters: []model.Filter{{
				Name:   "drop-all",
				Action: model.FilterAction{Mode: syslogserver.ActionDropSilent},
			}},
		}},
		IdempotencyKey: "live-filter",
	})
	if err != nil {
		t.Fatal(err)
	}
	sendUDP(t, addr.String(), []byte("<14>dropped-by-filter"))
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if svc.udp != nil && svc.udp.Metrics().DroppedFilter.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if svc.Messages().Stats().Messages != 1 {
		t.Fatalf("filter drop stored extra messages: %d", svc.Messages().Stats().Messages)
	}

	rev = svc.State(ctx).Revision
	_, err = svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: rev,
		Operations: []Operation{{
			Type:    OpReplaceFilters,
			Filters: []model.Filter{},
		}, {
			Type:     OpReplaceBehavior,
			Behavior: &model.Behavior{Mode: syslogserver.BehaviorDropSilent},
		}},
		IdempotencyKey: "live-behavior",
	})
	if err != nil {
		t.Fatal(err)
	}
	sendUDP(t, addr.String(), []byte("<14>dropped-by-behavior"))
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if svc.udp.Metrics().DroppedBehavior.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if svc.Messages().Stats().Messages != 1 {
		t.Fatalf("behavior drop stored extra messages: %d", svc.Messages().Stats().Messages)
	}
}

func TestServiceDoesNotWriteBootstrapIdentifiers(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != "os" {
				return true
			}
			switch sel.Sel.Name {
			case "WriteFile", "Create", "OpenFile":
				t.Errorf("%s: service must not %s the bootstrap file", path, sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func newTestService(t *testing.T, extraSpec string) *Service {
	t.Helper()
	dir := t.TempDir()
	tok := filepath.Join(dir, "token")
	if err := os.WriteFile(tok, bytes.Repeat([]byte("t"), auth.MinTokenBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `apiVersion: labsyslog.dev/v1alpha1
kind: LabSyslog
metadata:
  name: lab-sink
spec:
  listeners:
    udp:
      enabled: true
      address: "127.0.0.1:0"
    tcp:
      enabled: true
      address: "127.0.0.1:0"
  auth:
    tokens:
      - id: operator
        role: administrator
        secretFile: ` + tok + `
  admission:
    allowClientCidrs: ["127.0.0.0/8", "::1/128"]
` + extraSpec
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc, err := New(Config{BootstrapPath: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func sendUDP(t *testing.T, addr string, payload []byte) {
	t.Helper()
	c, err := net.Dial("udp4", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
}

func TestStateDriftedAfterApply(t *testing.T) {
	svc := newTestService(t, "")
	ctx := testutil.Context(t)
	if svc.State(ctx).Drifted {
		t.Fatal("fresh service should match bootstrap")
	}
	_, err := svc.Apply(ctx, ApplyRequest{
		ExpectedRevision: svc.State(ctx).Revision,
		Operations:       []Operation{{Type: OpReplaceObservability, LogLevel: "debug"}},
		IdempotencyKey:   "drift",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !svc.State(ctx).Drifted {
		t.Fatal("live apply should set drifted")
	}
}
