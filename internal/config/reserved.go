package config

import (
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

// Reserved prefixes after normalizing (strip - and _, lower-case).
var reservedPrefixes = []string{
	"forward",
	"relay",
	"remote",
	"destination",
	"smarthost",
	"outgoing",
	"output",
	"targethost",
	"remotehost",
	"omfwd",
	"rsyslog",
	"syslogng",
}

func normalizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func checkKey(key string) error {
	n := normalizeKey(key)
	if n == "secret" || n == "token" {
		return domainerr.Newf(domainerr.ReservedKey, "inline %s is not allowed", key)
	}
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(n, p) {
			return domainerr.Newf(domainerr.ReservedKey, "reserved key %q", key)
		}
	}
	if strings.Contains(key, "-") {
		return domainerr.Newf(domainerr.UnknownField, "kebab-case field names are not allowed: %s", key)
	}
	return nil
}
