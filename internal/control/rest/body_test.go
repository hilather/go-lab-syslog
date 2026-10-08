package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

// REST mutation bodies must be exactly one JSON value. A second value is
// validation_failed and must not run plan, apply, reset, clear, or wait.
func TestTrailingJSONRejectedBeforeMutation(t *testing.T) {
	ts, svc := newREST(t, "")
	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("<13>trailing")}); err != nil {
		t.Fatal(err)
	}
	before := svc.Messages().Stats().Messages
	rev := svc.Snapshot().Revision
	level := svc.Snapshot().Document.Spec.Observability.LogLevel

	t.Run("plan", func(t *testing.T) {
		auditBefore := len(svc.AuditRing().List())
		body := `{"expectedRevision":"` + rev + `","operations":[{"type":"replaceObservability","logLevel":"debug"}]}{"extra":true}`
		resp := postRaw(t, ts, "/v1/changes:plan", body)
		defer resp.Body.Close()
		assertTrailingJSON(t, resp)
		if got := len(svc.AuditRing().List()); got != auditBefore {
			t.Fatalf("plan trailing JSON grew the audit ring %d -> %d", auditBefore, got)
		}
		if got := svc.Snapshot().Document.Spec.Observability.LogLevel; got != level {
			t.Fatalf("plan changed logLevel %q -> %q", level, got)
		}
	})

	t.Run("apply", func(t *testing.T) {
		body := `{"expectedRevision":"` + rev + `","operations":[{"type":"replaceObservability","logLevel":"debug"}],"idempotencyKey":"trail-apply"}{"logLevel":"error"}`
		resp := postRaw(t, ts, "/v1/changes:apply", body)
		defer resp.Body.Close()
		assertTrailingJSON(t, resp)
		got := svc.Snapshot().Document.Spec.Observability.LogLevel
		if got != level {
			t.Fatalf("apply logLevel %q -> %q, want unchanged", level, got)
		}
	})

	t.Run("reset", func(t *testing.T) {
		resp := postRaw(t, ts, "/v1/state:reset", `{}{"extra":true}`)
		defer resp.Body.Close()
		assertTrailingJSON(t, resp)
		left := svc.Messages().Stats().Messages
		if left != before {
			t.Fatalf("reset messages %d -> %d, want store unchanged", before, left)
		}
	})

	t.Run("reset object", func(t *testing.T) {
		if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("<13>reset-object")}); err != nil {
			t.Fatal(err)
		}
		if svc.Messages().Stats().Messages == 0 {
			t.Fatal("store empty before {} reset")
		}
		resp := postRaw(t, ts, "/v1/state:reset", `{}`)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("{} reset status %d, want 200: %s", resp.StatusCode, b)
		}
		if left := svc.Messages().Stats().Messages; left != 0 {
			t.Fatalf("{} reset left %d messages, want the store wiped", left)
		}
	})

	t.Run("reset not json", func(t *testing.T) {
		if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("<13>reset-not-json")}); err != nil {
			t.Fatal(err)
		}
		stored := svc.Messages().Stats().Messages
		resp := postRaw(t, ts, "/v1/state:reset", `not-json`)
		defer resp.Body.Close()
		assertValidationFailed(t, resp)
		if left := svc.Messages().Stats().Messages; left != stored {
			t.Fatalf("not-json reset messages %d -> %d, want store unchanged", stored, left)
		}
	})

	t.Run("clear", func(t *testing.T) {
		if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Raw: []byte("<13>clear-trailing")}); err != nil {
			t.Fatal(err)
		}
		stored := svc.Messages().Stats().Messages
		resp := postRaw(t, ts, "/v1/messages:clear", `{}{"extra":true}`)
		defer resp.Body.Close()
		assertTrailingJSON(t, resp)
		if left := svc.Messages().Stats().Messages; left != stored {
			t.Fatalf("clear messages %d -> %d, want store unchanged", stored, left)
		}
	})

	t.Run("wait", func(t *testing.T) {
		resp := postRaw(t, ts, "/v1/messages:wait", `{"timeout":"1ms"}{"timeout":"1h"}`)
		defer resp.Body.Close()
		assertTrailingJSON(t, resp)
	})
}

func assertTrailingJSON(t *testing.T, resp *http.Response) {
	t.Helper()
	p := assertValidationFailed(t, resp)
	if !strings.Contains(p.Detail, "trailing JSON after document") {
		t.Fatalf("detail %q, want trailing JSON after document", p.Detail)
	}
}

func assertValidationFailed(t *testing.T, resp *http.Response) domainerr.Problem {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want 400: %s", resp.StatusCode, b)
	}
	var p domainerr.Problem
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("problem: %v body=%s", err, b)
	}
	if p.Code != domainerr.ValidationFailed {
		t.Fatalf("code %s, want validation_failed: %s", p.Code, b)
	}
	return p
}

func postRaw(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "trail-apply")
	setAuth(req)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
