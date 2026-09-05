package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

func TestWaitTimeout504(t *testing.T) {
	ts, _ := newREST(t, "")
	resp := postJSON(t, ts, "/v1/messages:wait", map[string]any{"timeout": "30ms"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status %d want 504", resp.StatusCode)
	}
	p := decodeProblem(t, resp)
	if p.Code != domainerr.WaitTimeout {
		t.Fatalf("code %s", p.Code)
	}
	if p.Type != "https://labsyslog.dev/errors/wait_timeout" {
		t.Fatalf("type %s", p.Type)
	}
}

func TestWaitWipe409(t *testing.T) {
	ts, svc := newREST(t, "")
	errc := make(chan *http.Response, 1)
	go func() {
		resp := postJSON(t, ts, "/v1/messages:wait", map[string]any{"timeout": "2s", "filter": map[string]any{"messageContains": "never"}})
		errc <- resp
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if svc.Messages().Stats().Waiters >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if svc.Messages().Stats().Waiters < 1 {
		t.Fatal("wait did not park")
	}
	clear := postJSON(t, ts, "/v1/messages:clear", nil)
	defer clear.Body.Close()
	if clear.StatusCode != http.StatusNoContent {
		t.Fatalf("clear %d", clear.StatusCode)
	}
	select {
	case resp := <-errc:
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d want 409 body=%s", resp.StatusCode, body)
		}
		p := decodeProblem(t, resp)
		if p.Code != domainerr.StoreWiped {
			t.Fatalf("code %s", p.Code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not return")
	}
}

func TestWaitExisting(t *testing.T) {
	ts, svc := newREST(t, "")
	if _, err := svc.Messages().Insert(model.Message{
		Transport: "udp",
		Parsed:    model.Parsed{Message: "hello-wait"},
	}); err != nil {
		t.Fatal(err)
	}
	resp := postJSON(t, ts, "/v1/messages:wait", map[string]any{
		"timeout": "1s",
		"filter":  map[string]any{"messageContains": "hello-wait"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["matched"] != "existing" {
		t.Fatalf("matched %v", body["matched"])
	}
}

func postJSON(t *testing.T, ts *httptest.Server, path string, payload any) *http.Response {
	t.Helper()
	var r io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	setAuth(req)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
