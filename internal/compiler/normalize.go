package compiler

import (
	"time"

	"github.com/hilather/go-lab-syslog/internal/model"
)

const (
	defaultUDPAddress       = ":514"
	defaultTCPAddress       = ":514"
	defaultTLSAddress       = ":6514"
	defaultRESTPath         = "/v1"
	defaultMCPPath          = "/mcp"
	defaultHostname         = "labsyslog.lab"
	defaultMaxMessages      = 10000
	defaultMaxDatagramsSec  = 20000
	defaultMaxDatagramsIP   = 2000
	defaultMaxTCPConns      = 256
	defaultMaxTCPConnsPerIP = 16
	defaultRPS              = 32
	defaultBurst            = 64
	defaultMaxConcurrent    = 256
	defaultAuditRing        = 128
	maxMessagesMin          = 1
	maxMessagesMax          = 1_000_000
)

var (
	defaultMaxMessageBytes = 64 * model.KiB
	defaultMaxBytes        = 256 * model.MiB
	defaultBodyLimit       = 1 * model.MiB
	defaultTCPIdle         = model.Duration(2 * time.Minute)
	defaultSessionTimeout  = model.Duration(2 * time.Minute)
	defaultMaxWait         = model.Duration(60 * time.Second)
)

// DefaultAllowClientCIDRs is applied when admission.allowClientCidrs is omitted.
var DefaultAllowClientCIDRs = []string{"127.0.0.0/8", "::1/128"}

// Normalize materializes 1.0 defaults in place. Empty allowClientCidrs is
// left empty so Validate can reject it; omitted (nil) becomes the loopback default.
func Normalize(doc *model.Document) {
	if doc == nil {
		return
	}
	s := &doc.Spec

	setBoolDefault(&s.Listeners.UDP.Enabled, true)
	if s.Listeners.UDP.Address == "" {
		s.Listeners.UDP.Address = defaultUDPAddress
	}
	setBoolDefault(&s.Listeners.TCP.Enabled, true)
	if s.Listeners.TCP.Address == "" {
		s.Listeners.TCP.Address = defaultTCPAddress
	}
	if s.Listeners.TCP.Framing == "" {
		s.Listeners.TCP.Framing = "auto"
	}
	if s.Listeners.TLS.Address == "" {
		s.Listeners.TLS.Address = defaultTLSAddress
	}
	if s.Listeners.Management.RESTPath == "" {
		s.Listeners.Management.RESTPath = defaultRESTPath
	}
	if s.Listeners.Management.MCPPath == "" {
		s.Listeners.Management.MCPPath = defaultMCPPath
	}

	if s.Auth.Mode == "" {
		s.Auth.Mode = "bearer"
	}
	for i := range s.Auth.Tokens {
		if s.Auth.Tokens[i].Role == "" {
			s.Auth.Tokens[i].Role = "administrator"
		}
	}

	setBoolDefault(&s.UI.Enabled, true)

	if s.Management.BodyLimit == 0 {
		s.Management.BodyLimit = defaultBodyLimit
	}
	if s.Management.RequestsPerSecond == 0 {
		s.Management.RequestsPerSecond = defaultRPS
	}
	if s.Management.Burst == 0 {
		s.Management.Burst = defaultBurst
	}
	if s.Management.MaxConcurrent == 0 {
		s.Management.MaxConcurrent = defaultMaxConcurrent
	}

	if s.Syslog.Hostname == "" {
		s.Syslog.Hostname = defaultHostname
	}
	setBoolDefault(&s.Syslog.Parse.RFC3164, true)
	setBoolDefault(&s.Syslog.Parse.RFC5424, true)
	setBoolDefault(&s.Syslog.Parse.BestEffort, true)
	if s.Syslog.MaxMessageBytes == 0 {
		s.Syslog.MaxMessageBytes = defaultMaxMessageBytes
	}
	if s.Syslog.UDPMaxDatagramBytes == 0 {
		s.Syslog.UDPMaxDatagramBytes = defaultMaxMessageBytes
	}
	if s.Syslog.TCPIdleTimeout == 0 {
		s.Syslog.TCPIdleTimeout = defaultTCPIdle
	}
	if s.Syslog.Behavior.Mode == "" {
		s.Syslog.Behavior.Mode = "accept"
	}

	if s.Admission.AllowClientCIDRs == nil {
		s.Admission.AllowClientCIDRs = append([]string(nil), DefaultAllowClientCIDRs...)
	}
	if s.Admission.MaxDatagramsPerSec == 0 {
		s.Admission.MaxDatagramsPerSec = defaultMaxDatagramsSec
	}
	if s.Admission.MaxDatagramsPerIP == 0 {
		s.Admission.MaxDatagramsPerIP = defaultMaxDatagramsIP
	}
	if s.Admission.MaxTCPConns == 0 {
		s.Admission.MaxTCPConns = defaultMaxTCPConns
	}
	if s.Admission.MaxTCPConnsPerIP == 0 {
		s.Admission.MaxTCPConnsPerIP = defaultMaxTCPConnsPerIP
	}
	if s.Admission.SessionTimeout == 0 {
		s.Admission.SessionTimeout = defaultSessionTimeout
	}

	if s.Store.MaxMessages == 0 {
		s.Store.MaxMessages = defaultMaxMessages
	}
	if s.Store.MaxBytes == 0 {
		s.Store.MaxBytes = defaultMaxBytes
	}
	if s.Store.FullPolicy == "" {
		s.Store.FullPolicy = "evict_oldest"
	}
	if s.Store.MaxWait == 0 {
		s.Store.MaxWait = defaultMaxWait
	}
	setBoolDefault(&s.Store.RawRetain, true)

	if s.Filters == nil {
		s.Filters = []model.Filter{}
	}
	for i := range s.Filters {
		setBoolDefault(&s.Filters[i].Enabled, true)
		if s.Filters[i].Action.Mode == "" {
			s.Filters[i].Action.Mode = "capture"
		}
	}

	if s.Observability.LogLevel == "" {
		s.Observability.LogLevel = "info"
	}
	if s.Observability.Audit.Ring == 0 {
		s.Observability.Audit.Ring = defaultAuditRing
	}
}

func setBoolDefault(p **bool, v bool) {
	if *p == nil {
		b := v
		*p = &b
	}
}
