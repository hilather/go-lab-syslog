package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestSessionCookieAndCSRF(t *testing.T) {
	s := NewStore(DefaultSessionConfig())
	p := Principal{ID: "operator", Class: ClassToken, Role: RoleAdministrator, Scopes: DefaultScopes(RoleAdministrator)}
	cookie, csrf, sess, err := s.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if cookie == "" || csrf == "" || sess.TokenID != "operator" {
		t.Fatal(sess)
	}
	got, gotCSRF, ok := s.Lookup(cookie)
	if !ok || got.TokenID != "operator" || gotCSRF != csrf {
		t.Fatal(got)
	}
	if !s.ValidCSRF(cookie, csrf) {
		t.Fatal("csrf")
	}
	if s.ValidCSRF(cookie, "nope") {
		t.Fatal("bad csrf")
	}
	c := NewSessionCookie(cookie, false, s.MaxAge())
	if c.Name != CookieName || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Fatalf("%+v", c)
	}
	if CookieName != "labsyslog_session" || CSRFHeader != "X-LabSyslog-CSRF" {
		t.Fatal(CookieName, CSRFHeader)
	}
}

func TestViewDoesNotSlideAndDropsExpired(t *testing.T) {
	s := NewStore(SessionConfig{Idle: time.Hour, Absolute: 2 * time.Hour, Max: 4})
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	p := Principal{ID: "operator", Class: ClassToken, Role: RoleAdministrator, Scopes: DefaultScopes(RoleAdministrator)}
	cookie, _, _, err := s.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.View(cookie)
	if !ok || !got.LastSeen.Equal(now) {
		t.Fatalf("view %+v ok=%v", got, ok)
	}
	// One second later is still inside the idle window. LastSeen must
	// stay at creation time, so a View that assigns LastSeen = now() fails.
	s.now = func() time.Time { return now.Add(time.Second) }
	again, ok := s.View(cookie)
	if !ok || !again.LastSeen.Equal(now) {
		t.Fatalf("view slid LastSeen %s -> %s", now, again.LastSeen)
	}
	s.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, ok := s.View(cookie); ok {
		t.Fatal("expired session still visible")
	}
	s.mu.Lock()
	left := len(s.sessions)
	s.mu.Unlock()
	if left != 0 {
		t.Fatalf("View left %d expired session", left)
	}
}

func TestExpiryAndEvictionNotifyDeleted(t *testing.T) {
	p := Principal{ID: "operator", Class: ClassToken, Role: RoleAdministrator, Scopes: DefaultScopes(RoleAdministrator)}
	base := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	type hit struct {
		n    int
		held bool
	}
	arm := func(s *Store) *hit {
		h := &hit{}
		s.OnDelete(func() {
			if !s.mu.TryLock() {
				h.held = true
				return
			}
			s.mu.Unlock()
			h.n++
		})
		return h
	}
	check := func(t *testing.T, h *hit) {
		t.Helper()
		if h.held {
			t.Fatal("onDelete ran while the store lock was held")
		}
		if h.n != 1 {
			t.Fatalf("notifications = %d, want 1", h.n)
		}
	}
	newClocked := func(idle, absolute time.Duration, max int) (*Store, func(time.Duration)) {
		s := NewStore(SessionConfig{Idle: idle, Absolute: absolute, Max: max})
		clock := base
		s.now = func() time.Time { return clock }
		return s, func(d time.Duration) { clock = clock.Add(d) }
	}
	left := func(s *Store) int {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.sessions)
	}

	t.Run("lookup idle", func(t *testing.T) {
		s, advance := newClocked(time.Hour, 4*time.Hour, 4)
		h := arm(s)
		cookie, _, _, err := s.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		advance(time.Hour + time.Second)
		if _, _, ok := s.Lookup(cookie); ok {
			t.Fatal("idle-expired session still present")
		}
		check(t, h)
		if left(s) != 0 {
			t.Fatalf("sessions left = %d", left(s))
		}
	})

	t.Run("lookup absolute", func(t *testing.T) {
		s, advance := newClocked(4*time.Hour, time.Hour, 4)
		h := arm(s)
		cookie, _, _, err := s.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		advance(time.Hour + time.Second)
		if _, _, ok := s.Lookup(cookie); ok {
			t.Fatal("absolute-expired session still present")
		}
		check(t, h)
	})

	t.Run("view idle", func(t *testing.T) {
		s, advance := newClocked(time.Hour, 4*time.Hour, 4)
		h := arm(s)
		cookie, _, _, err := s.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		advance(time.Hour + time.Second)
		if _, ok := s.View(cookie); ok {
			t.Fatal("idle-expired session still visible")
		}
		check(t, h)
	})

	t.Run("validcsrf idle", func(t *testing.T) {
		s, advance := newClocked(time.Hour, 4*time.Hour, 4)
		h := arm(s)
		cookie, csrf, _, err := s.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		advance(time.Hour + time.Second)
		if s.ValidCSRF(cookie, csrf) {
			t.Fatal("idle-expired session still accepted csrf")
		}
		check(t, h)
	})

	t.Run("create expire", func(t *testing.T) {
		s, advance := newClocked(time.Hour, 4*time.Hour, 4)
		h := arm(s)
		if _, _, _, err := s.Create(p); err != nil {
			t.Fatal(err)
		}
		advance(time.Hour + time.Second)
		if _, _, _, err := s.Create(p); err != nil {
			t.Fatal(err)
		}
		check(t, h)
		if left(s) != 1 {
			t.Fatalf("sessions left = %d, want the new one", left(s))
		}
	})

	t.Run("create evict", func(t *testing.T) {
		s, _ := newClocked(time.Hour, 4*time.Hour, 1)
		h := arm(s)
		if _, _, _, err := s.Create(p); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := s.Create(p); err != nil {
			t.Fatal(err)
		}
		check(t, h)
		if left(s) != 1 {
			t.Fatalf("sessions left = %d, want the new one", left(s))
		}
	})
}

func TestOriginAllowlist(t *testing.T) {
	if err := CheckOrigin("", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("http://127.0.0.1:8088", nil); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("https://evil.example", nil); err == nil {
		t.Fatal("non-loopback origin must be denied")
	}
	if err := CheckOrigin("https://lab.example", []string{"https://lab.example"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckOrigin("http://127.0.0.1:8088", []string{"https://lab.example"}); err == nil {
		t.Fatal("non-empty allowlist must not union loopback")
	}
	if err := CheckOrigin("http://localhost:18514", []string{"https://lab.example"}); err == nil {
		t.Fatal("localhost not listed")
	}
	if err := CheckOrigin("file://tmp", nil); err == nil {
		t.Fatal("file://")
	}
	if err := CheckOrigin("https://evil.example", []string{"*"}); err == nil {
		t.Fatal("star sentinel must not allow")
	}
}
