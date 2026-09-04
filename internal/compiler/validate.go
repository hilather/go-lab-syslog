package compiler

import (
	"bytes"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/hilather/go-lab-syslog/internal/auth"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
	"github.com/hilather/go-lab-syslog/internal/model"
)

var dnsLabelRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

var facilities = map[string]struct{}{
	"kern": {}, "user": {}, "mail": {}, "daemon": {}, "auth": {},
	"syslog": {}, "lpr": {}, "news": {}, "uucp": {}, "cron": {},
	"authpriv": {}, "ftp": {}, "ntp": {}, "audit": {}, "console": {},
	"cron2":  {},
	"local0": {}, "local1": {}, "local2": {}, "local3": {},
	"local4": {}, "local5": {}, "local6": {}, "local7": {},
}

var severities = map[string]struct{}{
	"emerg": {}, "alert": {}, "crit": {}, "err": {},
	"warning": {}, "notice": {}, "info": {}, "debug": {},
}

var (
	framingModes  = map[string]struct{}{"auto": {}, "octet-counting": {}, "non-transparent": {}}
	fullPolicies  = map[string]struct{}{"evict_oldest": {}, "reject": {}}
	behaviorModes = map[string]struct{}{"accept": {}, "drop-silent": {}, "close": {}}
	authModes     = map[string]struct{}{"bearer": {}}
	tokenRoles    = map[string]struct{}{"administrator": {}, "reader": {}}
	actionModes   = map[string]struct{}{"capture": {}, "drop-silent": {}, "tag": {}}
	logLevels     = map[string]struct{}{"debug": {}, "info": {}, "warn": {}, "error": {}}
	transports    = map[string]struct{}{"udp": {}, "tcp": {}}
)

// Check normalizes then validates. configDir resolves relative secretFile paths.
func Check(doc *model.Document, configDir string) error {
	Normalize(doc)
	return Validate(doc, configDir)
}

// Validate checks a normalized document. Missing token files are allowed;
// an existing file whose trimmed contents are shorter than auth.MinTokenBytes
// is rejected.
func Validate(doc *model.Document, configDir string) error {
	if doc == nil {
		return domainerr.New(domainerr.ValidationFailed, "document is empty")
	}
	if doc.APIVersion != model.APIVersion {
		return domainerr.Newf(domainerr.ValidationFailed, "apiVersion must be %s", model.APIVersion)
	}
	if doc.Kind != model.Kind {
		return domainerr.Newf(domainerr.ValidationFailed, "kind must be %s", model.Kind)
	}
	if !dnsLabelRE.MatchString(doc.Metadata.Name) {
		return domainerr.Newf(domainerr.ValidationFailed, "metadata.name %q is not a DNS label", doc.Metadata.Name)
	}

	s := doc.Spec
	if err := checkAddr("spec.listeners.udp.address", s.Listeners.UDP.Address); err != nil {
		return err
	}
	if err := checkAddr("spec.listeners.tcp.address", s.Listeners.TCP.Address); err != nil {
		return err
	}
	if _, ok := framingModes[s.Listeners.TCP.Framing]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.listeners.tcp.framing must be auto|octet-counting|non-transparent, got %q", s.Listeners.TCP.Framing)
	}
	if s.Listeners.TLS.Enabled {
		return domainerr.New(domainerr.TLSUnsupported, "spec.listeners.tls.enabled is not supported in 1.0")
	}
	if s.Listeners.TLS.Address != "" {
		if err := checkAddr("spec.listeners.tls.address", s.Listeners.TLS.Address); err != nil {
			return err
		}
	}
	if s.Listeners.Management.Address != "" {
		if err := checkAddr("spec.listeners.management.address", s.Listeners.Management.Address); err != nil {
			return err
		}
	}

	if _, ok := authModes[s.Auth.Mode]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.auth.mode must be bearer, got %q", s.Auth.Mode)
	}
	seenTokenID := map[string]struct{}{}
	for i, tok := range s.Auth.Tokens {
		if tok.ID == "" {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.auth.tokens[%d].id is required", i)
		}
		if _, dup := seenTokenID[tok.ID]; dup {
			return domainerr.Newf(domainerr.ValidationFailed, "duplicate token id %q", tok.ID)
		}
		seenTokenID[tok.ID] = struct{}{}
		if _, ok := tokenRoles[tok.Role]; !ok {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.auth.tokens[%d].role must be administrator|reader, got %q", i, tok.Role)
		}
		if tok.SecretFile == "" {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.auth.tokens[%d].secretFile is required", i)
		}
		if err := checkTokenFile(tok.SecretFile, configDir); err != nil {
			return err
		}
	}

	if s.Syslog.Parse.RFC3164 == nil || s.Syslog.Parse.RFC5424 == nil {
		return domainerr.New(domainerr.ValidationFailed, "syslog.parse parsers are not normalized")
	}
	if !*s.Syslog.Parse.RFC3164 && !*s.Syslog.Parse.RFC5424 {
		return domainerr.New(domainerr.ValidationFailed, "at least one of spec.syslog.parse.rfc3164 or rfc5424 must be true")
	}
	if s.Syslog.MaxMessageBytes <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.syslog.maxMessageBytes must be > 0")
	}
	if s.Syslog.UDPMaxDatagramBytes <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.syslog.udpMaxDatagramBytes must be > 0")
	}
	if s.Syslog.TCPIdleTimeout <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.syslog.tcpIdleTimeout must be > 0")
	}
	if _, ok := behaviorModes[s.Syslog.Behavior.Mode]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.syslog.behavior.mode must be accept|drop-silent|close, got %q", s.Syslog.Behavior.Mode)
	}

	if len(s.Admission.AllowClientCIDRs) == 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.admission.allowClientCidrs must be non-empty")
	}
	for _, c := range s.Admission.AllowClientCIDRs {
		if err := checkCIDR("spec.admission.allowClientCidrs", c); err != nil {
			return err
		}
	}
	if s.Admission.MaxDatagramsPerSec <= 0 || s.Admission.MaxDatagramsPerIP <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.admission datagram caps must be > 0")
	}
	if s.Admission.MaxTCPConns <= 0 || s.Admission.MaxTCPConnsPerIP <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.admission TCP caps must be > 0")
	}
	if s.Admission.SessionTimeout <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.admission.sessionTimeout must be > 0")
	}

	if s.Store.MaxMessages < maxMessagesMin || s.Store.MaxMessages > maxMessagesMax {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.store.maxMessages must be between %d and %d", maxMessagesMin, maxMessagesMax)
	}
	if s.Store.MaxBytes < 64*model.KiB {
		return domainerr.New(domainerr.ValidationFailed, "spec.store.maxBytes must be at least 64KiB")
	}
	if _, ok := fullPolicies[s.Store.FullPolicy]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.store.fullPolicy must be evict_oldest|reject, got %q", s.Store.FullPolicy)
	}
	if s.Store.MaxWait <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.store.maxWait must be > 0")
	}

	seenFilter := map[string]struct{}{}
	for i, f := range s.Filters {
		if !dnsLabelRE.MatchString(f.Name) {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].name %q is not a DNS label", i, f.Name)
		}
		if _, dup := seenFilter[f.Name]; dup {
			return domainerr.Newf(domainerr.ValidationFailed, "duplicate filter name %q", f.Name)
		}
		seenFilter[f.Name] = struct{}{}
		if err := checkFilter(i, f); err != nil {
			return err
		}
	}

	for _, o := range s.Management.AllowedOrigins {
		if o == "*" || o == "private" {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.management.allowedOrigins does not allow %q in 1.0", o)
		}
	}
	if s.Management.BodyLimit <= 0 || s.Management.RequestsPerSecond <= 0 || s.Management.Burst <= 0 || s.Management.MaxConcurrent <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.management HTTP limits must be > 0")
	}

	if _, ok := logLevels[s.Observability.LogLevel]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.observability.logLevel must be debug|info|warn|error, got %q", s.Observability.LogLevel)
	}
	if s.Observability.Audit.Ring <= 0 {
		return domainerr.New(domainerr.ValidationFailed, "spec.observability.audit.ring must be > 0")
	}
	return nil
}

func checkFilter(i int, f model.Filter) error {
	for _, c := range f.Match.SourceCIDRs {
		if err := checkCIDR("spec.filters.match.sourceCidrs", c); err != nil {
			return err
		}
	}
	for _, fac := range f.Match.Facilities {
		if err := checkFacility(fac); err != nil {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].match.facilities: %s", i, err.Error())
		}
	}
	for _, sev := range f.Match.Severities {
		if err := checkSeverity(sev); err != nil {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].match.severities: %s", i, err.Error())
		}
	}
	if f.Match.SeverityAtLeast != "" {
		if len(f.Match.Severities) > 0 {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].match cannot set both severities and severityAtLeast", i)
		}
		if err := checkSeverity(f.Match.SeverityAtLeast); err != nil {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].match.severityAtLeast: %s", i, err.Error())
		}
	}
	for _, tr := range f.Match.Transports {
		if _, ok := transports[tr]; !ok {
			return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].match.transports must be udp|tcp, got %q", i, tr)
		}
	}
	if _, ok := actionModes[f.Action.Mode]; !ok {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].action.mode must be capture|drop-silent|tag, got %q", i, f.Action.Mode)
	}
	if f.Action.Mode == "tag" && f.Action.Tag == "" {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].action.tag is required when mode is tag", i)
	}
	if f.Action.Mode != "tag" && f.Action.Tag != "" {
		return domainerr.Newf(domainerr.ValidationFailed, "spec.filters[%d].action.tag is only valid when mode is tag", i)
	}
	return nil
}

func checkTokenFile(path, configDir string) error {
	resolved := path
	if !filepath.IsAbs(path) && configDir != "" {
		resolved = filepath.Join(configDir, path)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domainerr.Newf(domainerr.ValidationFailed, "secretFile %q: %v", path, err)
	}
	if len(bytes.TrimSpace(data)) < auth.MinTokenBytes {
		return domainerr.Newf(domainerr.ValidationFailed, "secretFile %q trimmed contents are shorter than %d bytes", path, auth.MinTokenBytes)
	}
	return nil
}

func checkAddr(field, addr string) error {
	if addr == "" {
		return domainerr.Newf(domainerr.ValidationFailed, "%s is required", field)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return domainerr.Newf(domainerr.ValidationFailed, "%s %q is not a host:port listen address", field, addr)
	}
	return nil
}

func checkCIDR(field, cidr string) error {
	if _, err := netip.ParsePrefix(cidr); err != nil {
		if _, _, err2 := net.ParseCIDR(cidr); err2 != nil {
			return domainerr.Newf(domainerr.ValidationFailed, "%s %q is not a CIDR", field, cidr)
		}
	}
	return nil
}

func checkFacility(v string) error {
	if _, ok := facilities[v]; ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 23 {
		return domainerr.Newf(domainerr.ValidationFailed, "unknown facility %q", v)
	}
	return nil
}

func checkSeverity(v string) error {
	if _, ok := severities[v]; ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 7 {
		return domainerr.Newf(domainerr.ValidationFailed, "unknown severity %q", v)
	}
	return nil
}
