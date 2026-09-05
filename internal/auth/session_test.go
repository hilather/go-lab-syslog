package auth

import (
	"net/http"
	"testing"
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
