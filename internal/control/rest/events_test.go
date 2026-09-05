package rest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func TestEventsStreamReceived(t *testing.T) {
	svc := newService(t, "")
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	s := newServer(svc)
	s.heartbeat = 20 * time.Millisecond
	ts := newTestServer(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "sse"}}); err != nil {
		t.Fatal(err)
	}

	var buf []byte
	tmp := make([]byte, 256)
	gotReceived := false
	gotHeartbeat := false
	for !gotReceived || !gotHeartbeat {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			text := string(buf)
			if strings.Contains(text, store.EventReceived) {
				gotReceived = true
			}
			if strings.Contains(text, "heartbeat") {
				gotHeartbeat = true
			}
		}
		if err != nil {
			if err == io.EOF || ctx.Err() != nil {
				break
			}
			break
		}
	}
	if !gotReceived {
		t.Fatalf("missing syslog.received in %q", buf)
	}
	if !gotHeartbeat {
		t.Fatalf("missing heartbeat in %q", buf)
	}
}

func TestEventsStreamDeletedAndWiped(t *testing.T) {
	svc := newService(t, "")
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	s := newServer(svc)
	s.heartbeat = time.Hour
	ts := newTestServer(t, s)

	if _, err := svc.Messages().Insert(model.Message{Transport: "udp", Parsed: model.Parsed{Message: "to-delete"}}); err != nil {
		t.Fatal(err)
	}
	listed, err := svc.Messages().List(store.ListFilter{}, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("stored %d", len(listed.Items))
	}
	id := listed.Items[0].ID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}

	del := make(chan *http.Response, 1)
	go func() {
		r, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/messages/"+id, nil)
		if err != nil {
			t.Error(err)
			return
		}
		out, err := ts.Client().Do(r)
		if err != nil {
			t.Error(err)
			return
		}
		del <- out
	}()

	text := readSSEUntil(t, ctx, resp.Body, store.EventDeleted)
	select {
	case out := <-del:
		defer out.Body.Close()
		if out.StatusCode != http.StatusNoContent {
			t.Fatalf("delete %d", out.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("delete did not return")
	}
	if !strings.Contains(text, "event: "+store.EventDeleted) {
		t.Fatalf("missing syslog.deleted in %q", text)
	}

	clearDone := make(chan *http.Response, 1)
	go func() {
		clearDone <- postJSON(t, ts, "/v1/messages:clear", nil)
	}()
	text = readSSEUntil(t, ctx, resp.Body, store.EventWiped)
	select {
	case out := <-clearDone:
		defer out.Body.Close()
		if out.StatusCode != http.StatusNoContent {
			t.Fatalf("clear %d", out.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("clear did not return")
	}
	if !strings.Contains(text, "event: "+store.EventWiped) {
		t.Fatalf("missing store.wiped in %q", text)
	}
}

func readSSEUntil(t *testing.T, ctx context.Context, r io.Reader, kind string) string {
	t.Helper()
	var buf []byte
	tmp := make([]byte, 256)
	for {
		if ctx.Err() != nil {
			t.Fatalf("timeout waiting for %s in %q", kind, buf)
		}
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if strings.Contains(string(buf), "event: "+kind) {
				return string(buf)
			}
		}
		if err != nil {
			t.Fatalf("read waiting for %s: %v buf=%q", kind, err, buf)
		}
	}
}
