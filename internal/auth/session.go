package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

const (
	// CookieName is the REST-only UI session cookie.
	CookieName = "labsyslog_session"
	// CSRFHeader is required on cookie-authenticated mutations.
	CSRFHeader = "X-LabSyslog-CSRF"
	cookiePath = "/"
)

// SessionConfig sizes the process-local session table.
type SessionConfig struct {
	Idle     time.Duration
	Absolute time.Duration
	Max      int
}

// DefaultSessionConfig is TTL 12h, idle 4h, max 64.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		Idle:     4 * time.Hour,
		Absolute: 12 * time.Hour,
		Max:      64,
	}
}

// Session is the public, non-secret view of an in-memory session.
type Session struct {
	ID        string
	TokenID   string
	Role      string
	Scopes    []string
	CreatedAt time.Time
	LastSeen  time.Time
}

// Store is a process-local session table. Cookie values and CSRF secrets
// stay in memory and are never persisted. REST-only; MCP is bearer-only.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*sessionRecord
	cfg      SessionConfig
	now      func() time.Time
	onDelete func()
}

type sessionRecord struct {
	public    Session
	csrf      string
	createdAt time.Time
	lastSeen  time.Time
}

// NewStore builds a session table. Non-positive durations use the defaults.
func NewStore(cfg SessionConfig) *Store {
	def := DefaultSessionConfig()
	if cfg.Idle <= 0 {
		cfg.Idle = def.Idle
	}
	if cfg.Absolute <= 0 {
		cfg.Absolute = def.Absolute
	}
	if cfg.Max <= 0 {
		cfg.Max = def.Max
	}
	return &Store{
		sessions: make(map[string]*sessionRecord),
		cfg:      cfg,
		now:      time.Now,
	}
}

// Create issues a new session and CSRF secret.
func (s *Store) Create(p Principal) (cookieValue, csrf string, sess Session, err error) {
	if s == nil {
		return "", "", Session{}, domainerr.New(domainerr.ValidationFailed, "session store unavailable")
	}
	cookieValue, err = randomHex(MinTokenBytes)
	if err != nil {
		return "", "", Session{}, err
	}
	csrf, err = randomHex(MinTokenBytes)
	if err != nil {
		return "", "", Session{}, err
	}
	publicID, err := randomHex(16)
	if err != nil {
		return "", "", Session{}, err
	}
	now := s.currentTime()
	rec := &sessionRecord{
		public: Session{
			ID:        publicID,
			TokenID:   p.ID,
			Role:      p.Role,
			Scopes:    append([]string(nil), p.Scopes...),
			CreatedAt: now,
			LastSeen:  now,
		},
		csrf:      csrf,
		createdAt: now,
		lastSeen:  now,
	}
	s.mu.Lock()
	removed := s.expireLocked(now)
	if len(s.sessions) >= s.cfg.Max && s.evictOldestLocked() {
		removed = true
	}
	s.sessions[cookieValue] = rec
	public := rec.public
	s.mu.Unlock()
	if removed {
		s.notifyDeleted()
	}
	return cookieValue, csrf, public, nil
}

// Lookup returns the session for cookieValue and touches LastSeen.
func (s *Store) Lookup(cookieValue string) (Session, string, bool) {
	if s == nil || cookieValue == "" {
		return Session{}, "", false
	}
	now := s.currentTime()
	s.mu.Lock()
	rec, ok := s.sessions[cookieValue]
	if !ok || s.expiredLocked(rec, now) {
		removed := ok
		if ok {
			delete(s.sessions, cookieValue)
		}
		s.mu.Unlock()
		if removed {
			s.notifyDeleted()
		}
		return Session{}, "", false
	}
	rec.lastSeen = now
	rec.public.LastSeen = now
	public, csrf := rec.public, rec.csrf
	s.mu.Unlock()
	return public, csrf, true
}

// View returns the session for cookieValue without sliding LastSeen.
// An expired session is deleted, the same as Lookup.
func (s *Store) View(cookieValue string) (Session, bool) {
	if s == nil || cookieValue == "" {
		return Session{}, false
	}
	now := s.currentTime()
	s.mu.Lock()
	rec, ok := s.sessions[cookieValue]
	if !ok || s.expiredLocked(rec, now) {
		removed := ok
		if ok {
			delete(s.sessions, cookieValue)
		}
		s.mu.Unlock()
		if removed {
			s.notifyDeleted()
		}
		return Session{}, false
	}
	public := rec.public
	s.mu.Unlock()
	return public, true
}

// SetNow replaces the clock used for idle and absolute expiry.
// Tests inject a clock this way. Nil restores time.Now.
func (s *Store) SetNow(now func() time.Time) {
	if s == nil {
		return
	}
	if now == nil {
		now = time.Now
	}
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// OnDelete registers fn, called after a session is removed.
// Delete, Clear, idle and absolute expiry, and max-session eviction
// each call it once, after the store lock is released. fn replaces
// any previous hook.
func (s *Store) OnDelete(fn func()) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.onDelete = fn
	s.mu.Unlock()
}

// Delete removes one cookie session.
func (s *Store) Delete(cookieValue string) {
	if s == nil || cookieValue == "" {
		return
	}
	s.mu.Lock()
	_, ok := s.sessions[cookieValue]
	delete(s.sessions, cookieValue)
	s.mu.Unlock()
	if ok {
		s.notifyDeleted()
	}
}

// Clear drops every session (reset).
func (s *Store) Clear() {
	if s == nil {
		return
	}
	s.mu.Lock()
	n := len(s.sessions)
	s.sessions = make(map[string]*sessionRecord)
	s.mu.Unlock()
	if n > 0 {
		s.notifyDeleted()
	}
}

func (s *Store) notifyDeleted() {
	s.mu.Lock()
	fn := s.onDelete
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// ValidCSRF compares the presented header to the session CSRF secret.
func (s *Store) ValidCSRF(cookieValue, presented string) bool {
	if s == nil || cookieValue == "" || presented == "" {
		return false
	}
	now := s.currentTime()
	s.mu.Lock()
	rec, ok := s.sessions[cookieValue]
	if !ok || s.expiredLocked(rec, now) {
		removed := ok
		if ok {
			delete(s.sessions, cookieValue)
		}
		s.mu.Unlock()
		if removed {
			s.notifyDeleted()
		}
		return false
	}
	match := EqualDigest(DigestSecret([]byte(rec.csrf)), DigestSecret([]byte(presented)))
	s.mu.Unlock()
	return match
}

func (s *Store) currentTime() time.Time {
	s.mu.Lock()
	fn := s.now
	s.mu.Unlock()
	if fn == nil {
		return time.Now()
	}
	return fn()
}

// MaxAge is the cookie Max-Age (absolute TTL).
func (s *Store) MaxAge() int {
	if s == nil {
		return int(DefaultSessionConfig().Absolute.Seconds())
	}
	return int(s.cfg.Absolute.Seconds())
}

// ExpiresAt is the earlier of idle and absolute expiry.
func (s *Store) ExpiresAt(sess Session) time.Time {
	if s == nil {
		return time.Time{}
	}
	idle := sess.LastSeen.Add(s.cfg.Idle)
	abs := sess.CreatedAt.Add(s.cfg.Absolute)
	if idle.Before(abs) {
		return idle
	}
	return abs
}

func (s *Store) expiredLocked(rec *sessionRecord, now time.Time) bool {
	if now.Sub(rec.lastSeen) > s.cfg.Idle {
		return true
	}
	return now.Sub(rec.createdAt) > s.cfg.Absolute
}

func (s *Store) expireLocked(now time.Time) bool {
	removed := false
	for k, rec := range s.sessions {
		if s.expiredLocked(rec, now) {
			delete(s.sessions, k)
			removed = true
		}
	}
	return removed
}

func (s *Store) evictOldestLocked() bool {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, rec := range s.sessions {
		if first || rec.lastSeen.Before(oldest) {
			oldestKey = k
			oldest = rec.lastSeen
			first = false
		}
	}
	if oldestKey == "" {
		return false
	}
	delete(s.sessions, oldestKey)
	return true
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", domainerr.New(domainerr.ValidationFailed, "session material unavailable")
	}
	return hex.EncodeToString(b), nil
}

// NewSessionCookie builds the browser cookie. Secure iff TLS (docs/08: not
// required on loopback HTTP).
func NewSessionCookie(value string, secure bool, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie expires the UI cookie.
func ClearSessionCookie(secure bool) *http.Cookie {
	c := NewSessionCookie("", secure, -1)
	c.Expires = time.Unix(0, 0).UTC()
	return c
}

// CookieSecure is true when the request is TLS.
func CookieSecure(r *http.Request) bool {
	return r != nil && r.TLS != nil
}

// UnsafeMethod is a cookie-CSRF-protected mutation.
func UnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// PrincipalFromSession copies the token principal off a cookie session.
func PrincipalFromSession(sess Session) Principal {
	return Principal{
		ID:     sess.TokenID,
		Class:  ClassSession,
		Role:   sess.Role,
		Scopes: append([]string(nil), sess.Scopes...),
	}
}
