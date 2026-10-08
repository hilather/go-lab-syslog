package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/testutil"
)

// An event stream authenticated with a cookie must stop delivering store
// events once that session is revoked.
func TestEventsStreamStopsAfterSessionRevocation(t *testing.T) {
	ts, svc := newREST(t, "")

	login, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(login)
	loginResp, err := ts.Client().Do(login)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login status %d body=%s", loginResp.StatusCode, b)
	}
	var sess struct {
		CSRF string `json:"csrf"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&sess); err != nil {
		t.Fatal(err)
	}
	cookie := sessionCookie(loginResp)
	if cookie == "" || sess.CSRF == "" {
		t.Fatal("missing cookie or csrf")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamResp.Body)
		t.Fatalf("stream status %d body=%s", streamResp.StatusCode, b)
	}

	var mu sync.Mutex
	var buf []byte
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		tmp := make([]byte, 512)
		for {
			n, err := streamResp.Body.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf = append(buf, tmp[:n]...)
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	logout, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	logout.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	logout.Header.Set(auth.CSRFHeader, sess.CSRF)
	logoutResp, err := ts.Client().Do(logout)
	if err != nil {
		t.Fatal(err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(logoutResp.Body)
		t.Fatalf("logout status %d body=%s", logoutResp.StatusCode, b)
	}

	id := insertMessageID(t, svc, "after-revoke")
	assertStreamClosedWithout(t, &mu, &buf, readDone, id)
}

// A bearer stream must stop once reset removes that secret. newREST binds
// management off, so the reset does not move a management socket.
func TestEventsStreamStopsAfterBearerRevocation(t *testing.T) {
	ts, svc := newREST(t, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(streamReq)
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamResp.Body)
		t.Fatalf("stream status %d body=%s", streamResp.StatusCode, b)
	}

	var mu sync.Mutex
	var buf []byte
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		tmp := make([]byte, 512)
		for {
			n, err := streamResp.Body.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf = append(buf, tmp[:n]...)
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	toks := svc.Snapshot().Document.Spec.Auth.Tokens
	if len(toks) != 1 || toks[0].SecretFile == "" {
		t.Fatalf("bootstrap token file missing: %+v", toks)
	}
	if err := os.WriteFile(toks[0].SecretFile, []byte(strings.Repeat("x", auth.MinTokenBytes)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reset(context.Background(), "operator", "revoke bearer"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Verifier().AuthenticateBearer(testBearerSecret); err == nil {
		t.Fatal("revoked bearer still authenticates")
	}

	id := insertMessageID(t, svc, "after-bearer-revoke")
	assertStreamClosedWithout(t, &mu, &buf, readDone, id)
}

// Heartbeats and delivered events must not extend the session idle timer.
func TestEventsStreamDoesNotSlideSessionIdle(t *testing.T) {
	svc := newService(t, "")
	if err := svc.Start(testutil.Context(t)); err != nil {
		t.Fatal(err)
	}
	s := newServer(svc)
	s.heartbeat = 200 * time.Millisecond
	ts := newTestServer(t, s)

	login, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	setAuth(login)
	loginResp, err := ts.Client().Do(login)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login status %d body=%s", loginResp.StatusCode, b)
	}
	cookie := sessionCookie(loginResp)
	if cookie == "" {
		t.Fatal("missing cookie")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	streamReq.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(streamResp.Body)
		t.Fatalf("stream status %d body=%s", streamResp.StatusCode, b)
	}
	opened, ok := svc.Sessions().View(cookie)
	if !ok {
		t.Fatal("session missing after the stream opened")
	}

	var mu sync.Mutex
	var buf []byte
	go func() {
		tmp := make([]byte, 512)
		for {
			n, err := streamResp.Body.Read(tmp)
			if n > 0 {
				mu.Lock()
				buf = append(buf, tmp[:n]...)
				mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	id := insertMessageID(t, svc, "idle-unchanged")
	deadline := time.Now().Add(2 * time.Second)
	var text string
	for {
		mu.Lock()
		text = string(buf)
		mu.Unlock()
		if strings.Contains(text, id) && strings.Contains(text, "heartbeat") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream did not deliver the event and a heartbeat: %s", text)
		}
		time.Sleep(10 * time.Millisecond)
	}
	again, ok := svc.Sessions().View(cookie)
	if !ok {
		t.Fatal("session missing after stream activity")
	}
	if !again.LastSeen.Equal(opened.LastSeen) {
		t.Fatalf("stream slid LastSeen %s -> %s", opened.LastSeen, again.LastSeen)
	}
}

func insertMessageID(t *testing.T, svc *app.Service, text string) string {
	t.Helper()
	if _, err := svc.Messages().Insert(model.Message{
		Transport: "udp",
		Parsed:    model.Parsed{Message: text},
	}); err != nil {
		t.Fatal(err)
	}
	listed, err := svc.Messages().List(store.ListFilter{}, "", 1)
	if err != nil || len(listed.Items) != 1 {
		t.Fatalf("list after insert: %v items=%d", err, len(listed.Items))
	}
	return listed.Items[0].ID
}

func assertStreamClosedWithout(t *testing.T, mu *sync.Mutex, buf *[]byte, readDone <-chan struct{}, id string) {
	t.Helper()
	deadline := time.Now().Add(800 * time.Millisecond)
	for {
		mu.Lock()
		text := string(*buf)
		mu.Unlock()
		if strings.Contains(text, id) {
			t.Fatalf("stream delivered id %s after revocation: %s", id, text)
		}
		select {
		case <-readDone:
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("stream still open after revocation; id %s was not refused", id)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
