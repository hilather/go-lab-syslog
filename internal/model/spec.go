package model

// Document is one labsyslog.dev/v1alpha1 object.
type Document struct {
	APIVersion string   `json:"apiVersion" yaml:"apiVersion"`
	Kind       string   `json:"kind" yaml:"kind"`
	Metadata   Metadata `json:"metadata" yaml:"metadata"`
	Spec       Spec     `json:"spec" yaml:"spec"`
}

// Metadata is document identity. Name is a DNS label.
type Metadata struct {
	Name string `json:"name" yaml:"name"`
}

// Spec is desired state. Wire names are frozen in docs/04.
type Spec struct {
	Listeners     Listeners     `json:"listeners" yaml:"listeners"`
	Auth          Auth          `json:"auth" yaml:"auth"`
	UI            UI            `json:"ui" yaml:"ui"`
	Management    Management    `json:"management" yaml:"management"`
	Syslog        Syslog        `json:"syslog" yaml:"syslog"`
	Admission     Admission     `json:"admission" yaml:"admission"`
	Store         Store         `json:"store" yaml:"store"`
	Filters       []Filter      `json:"filters" yaml:"filters"`
	Observability Observability `json:"observability" yaml:"observability"`
}

// Listeners is the bind surface. Addresses are reset-only.
type Listeners struct {
	UDP        UDPListener        `json:"udp" yaml:"udp"`
	TCP        TCPListener        `json:"tcp" yaml:"tcp"`
	TLS        TLSListener        `json:"tls" yaml:"tls"`
	Management ManagementListener `json:"management" yaml:"management"`
}

// UDPListener is RFC 5426 receive.
type UDPListener struct {
	Enabled *bool  `json:"enabled" yaml:"enabled"`
	Address string `json:"address" yaml:"address"`
}

// TCPListener is RFC 6587 receive.
type TCPListener struct {
	Enabled *bool  `json:"enabled" yaml:"enabled"`
	Address string `json:"address" yaml:"address"`
	Framing string `json:"framing" yaml:"framing"`
}

// TLSListener is the RFC 5425 placeholder. Fields other than enabled are
// ignored while enabled is false. enabled true is a validate error in 1.0.
type TLSListener struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	Address    string `json:"address" yaml:"address"`
	CertFile   string `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile    string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
	CAFile     string `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	ClientAuth bool   `json:"clientAuth" yaml:"clientAuth"`
}

// ManagementListener is the control-plane bind. Empty address means off
// unless a CLI flag sets one.
type ManagementListener struct {
	Address  string `json:"address,omitempty" yaml:"address,omitempty"`
	RESTPath string `json:"restPath" yaml:"restPath"`
	MCPPath  string `json:"mcpPath" yaml:"mcpPath"`
}

// Auth is bearer-only (ADR 0005). spec.management.auth is unknown.
type Auth struct {
	Mode   string      `json:"mode" yaml:"mode"`
	Tokens []AuthToken `json:"tokens,omitempty" yaml:"tokens,omitempty"`
}

// AuthToken is a file-ref bearer. Inline secret/token keys reject.
type AuthToken struct {
	ID         string `json:"id" yaml:"id"`
	Role       string `json:"role" yaml:"role"`
	SecretFile string `json:"secretFile" yaml:"secretFile"`
}

// UI is the operator SPA toggle.
type UI struct {
	Enabled *bool `json:"enabled" yaml:"enabled"`
}

// Management is HTTP limits, origins, and MCP flags. originAllowlist rejects.
type Management struct {
	AllowedOrigins    []string      `json:"allowedOrigins,omitempty" yaml:"allowedOrigins,omitempty"`
	MCP               ManagementMCP `json:"mcp" yaml:"mcp"`
	BodyLimit         ByteSize      `json:"bodyLimit" yaml:"bodyLimit"`
	RequestsPerSecond int           `json:"requestsPerSecond" yaml:"requestsPerSecond"`
	Burst             int           `json:"burst" yaml:"burst"`
	MaxConcurrent     int           `json:"maxConcurrent" yaml:"maxConcurrent"`
}

// ManagementMCP holds MCP adapter flags.
type ManagementMCP struct {
	AllowLegacyClients bool `json:"allowLegacyClients" yaml:"allowLegacyClients"`
}

// Syslog is parse policy and data-plane limits.
type Syslog struct {
	Hostname            string      `json:"hostname" yaml:"hostname"`
	Parse               SyslogParse `json:"parse" yaml:"parse"`
	MaxMessageBytes     ByteSize    `json:"maxMessageBytes" yaml:"maxMessageBytes"`
	UDPMaxDatagramBytes ByteSize    `json:"udpMaxDatagramBytes" yaml:"udpMaxDatagramBytes"`
	TCPIdleTimeout      Duration    `json:"tcpIdleTimeout" yaml:"tcpIdleTimeout"`
	Behavior            Behavior    `json:"behavior" yaml:"behavior"`
}

// SyslogParse selects RFC 3164 / 5424 parsers. At least one must be true.
type SyslogParse struct {
	RFC3164    *bool `json:"rfc3164" yaml:"rfc3164"`
	RFC5424    *bool `json:"rfc5424" yaml:"rfc5424"`
	BestEffort *bool `json:"bestEffort" yaml:"bestEffort"`
}

// Behavior is the deterministic ingest mode.
type Behavior struct {
	Mode string `json:"mode" yaml:"mode"`
}

// Admission is the ignore-outside CIDR gate and rate caps.
type Admission struct {
	AllowClientCIDRs   []string `json:"allowClientCidrs" yaml:"allowClientCidrs"`
	MaxDatagramsPerSec int      `json:"maxDatagramsPerSec" yaml:"maxDatagramsPerSec"`
	MaxDatagramsPerIP  int      `json:"maxDatagramsPerIP" yaml:"maxDatagramsPerIP"`
	MaxTCPConns        int      `json:"maxTcpConns" yaml:"maxTcpConns"`
	MaxTCPConnsPerIP   int      `json:"maxTcpConnsPerIP" yaml:"maxTcpConnsPerIP"`
	SessionTimeout     Duration `json:"sessionTimeout" yaml:"sessionTimeout"`
}

// Store is the ephemeral ring caps.
type Store struct {
	MaxMessages int      `json:"maxMessages" yaml:"maxMessages"`
	MaxBytes    ByteSize `json:"maxBytes" yaml:"maxBytes"`
	FullPolicy  string   `json:"fullPolicy" yaml:"fullPolicy"`
	MaxWait     Duration `json:"maxWait" yaml:"maxWait"`
	RawRetain   *bool    `json:"rawRetain" yaml:"rawRetain"`
}

// Filter is one first-match classifier. Duplicate Name is a validate error.
type Filter struct {
	Name    string       `json:"name" yaml:"name"`
	Enabled *bool        `json:"enabled" yaml:"enabled"`
	Match   FilterMatch  `json:"match" yaml:"match"`
	Action  FilterAction `json:"action" yaml:"action"`
}

// FilterMatch is AND of the listed predicates. Empty means unrestricted.
type FilterMatch struct {
	SourceCIDRs     []string `json:"sourceCidrs,omitempty" yaml:"sourceCidrs,omitempty"`
	Facilities      []string `json:"facilities,omitempty" yaml:"facilities,omitempty"`
	Severities      []string `json:"severities,omitempty" yaml:"severities,omitempty"`
	SeverityAtLeast string   `json:"severityAtLeast,omitempty" yaml:"severityAtLeast,omitempty"`
	AppNames        []string `json:"appNames,omitempty" yaml:"appNames,omitempty"`
	Hostnames       []string `json:"hostnames,omitempty" yaml:"hostnames,omitempty"`
	Transports      []string `json:"transports,omitempty" yaml:"transports,omitempty"`
}

// FilterAction is capture, drop-silent, or tag. Tag is required when mode is tag.
type FilterAction struct {
	Mode string `json:"mode" yaml:"mode"`
	Tag  string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// Observability is log level, metrics scrape, and audit ring size.
type Observability struct {
	LogLevel string             `json:"logLevel" yaml:"logLevel"`
	Metrics  Metrics            `json:"metrics" yaml:"metrics"`
	Audit    ObservabilityAudit `json:"audit" yaml:"audit"`
}

// Metrics controls GET /v1/metrics. metrics.listen is not a 1.0 field.
type Metrics struct {
	PublicPath bool `json:"publicPath" yaml:"publicPath"`
}

// ObservabilityAudit is the in-memory mutation ring.
type ObservabilityAudit struct {
	Ring int `json:"ring" yaml:"ring"`
}
