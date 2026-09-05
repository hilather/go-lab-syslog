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
