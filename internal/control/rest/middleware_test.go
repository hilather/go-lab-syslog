package rest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func TestBodyLimitPayloadTooLarge(t *testing.T) {
	ts, _ := newREST(t, `
  management:
    bodyLimit: 128B
`)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/state:validate", bytes.NewReader(bytes.Repeat([]byte("x"), 200)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want 413 body=%s", resp.StatusCode, body)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.PayloadTooLarge {
		t.Fatalf("code %s", p.Code)
	}
}

func TestRateLimitBurst(t *testing.T) {
	ts, _ := newREST(t, `
  management:
    requestsPerSecond: 1
    burst: 1
    maxConcurrent: 32
`)
	first := get(t, ts, "/v1/version")
	defer first.Body.Close()
	if first.StatusCode != 200 {
		t.Fatalf("first %d", first.StatusCode)
	}
	second := get(t, ts, "/v1/version")
	defer second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second %d want 429", second.StatusCode)
	}
	p := decodeProblem(t, second)
	if p.Code != domainerr.RateLimited {
		t.Fatalf("code %s", p.Code)
	}
}

func TestMaxConcurrentRateLimited(t *testing.T) {
	svc := newService(t, `
  management:
    requestsPerSecond: 100
    burst: 100
    maxConcurrent: 1
`)
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	s := newServer(svc)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)

	started := make(chan struct{})
	done := make(chan *http.Response, 1)
	go func() {
		close(started)
		done <- postJSON(t, ts, "/v1/messages:wait", map[string]any{"timeout": "1s"})
	}()
	<-started
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.inflight.Load() >= 1 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if s.inflight.Load() < 1 {
		t.Fatal("wait did not take a concurrent slot")
	}
	resp := get(t, ts, "/v1/version")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d want 429 body=%s", resp.StatusCode, body)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.RateLimited {
		t.Fatalf("code %s", p.Code)
	}
	waitResp := <-done
	defer waitResp.Body.Close()
}
