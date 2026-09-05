package auth

import (
	"net"
	"net/url"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

// CheckOrigin implements allowedOrigins exact match. Missing Origin is
// allowed (SDK/curl). Default empty list is loopback http(s) only. No
// "*" / "private" sentinels; file:// is denied.
func CheckOrigin(origin string, allowlist []string) error {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return domainerr.New(domainerr.OriginNotAllowed, "origin is not allowed")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return domainerr.New(domainerr.OriginNotAllowed, "origin is not allowed")
	}
	if isLoopbackHost(u.Hostname()) {
		return nil
	}
	for _, allowed := range allowlist {
		if originMatches(origin, allowed) {
			return nil
		}
	}
	return domainerr.New(domainerr.OriginNotAllowed, "origin is not allowed")
}

func originMatches(got, want string) bool {
	got = strings.TrimRight(strings.TrimSpace(got), "/")
	want = strings.TrimRight(strings.TrimSpace(want), "/")
	return strings.EqualFold(got, want)
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
