package model

import (
	"net/netip"
	"time"
)

// Message is one stored syslog record. Truncated is always false in 1.0
// because oversize datagrams and frames are dropped, not rewritten.
type Message struct {
	ID           string     `json:"id"`
	ReceivedAt   time.Time  `json:"receivedAt"`
	Transport    string     `json:"transport"`
	RemoteIP     netip.Addr `json:"remoteIP"`
	RemotePort   uint16     `json:"remotePort"`
	Raw          []byte     `json:"raw,omitempty"`
	Truncated    bool       `json:"truncated"`
	ParseWarning string     `json:"parseWarning,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	Parsed       Parsed     `json:"parsed"`
}

// Parsed is the best-effort structured view of a syslog message.
type Parsed struct {
	PRI        uint8       `json:"pri"`
	Facility   uint8       `json:"facility"`
	Severity   uint8       `json:"severity"`
	Version    uint8       `json:"version"`
	Timestamp  time.Time   `json:"timestamp"`
	Hostname   string      `json:"hostname,omitempty"`
	AppName    string      `json:"appName,omitempty"`
	ProcID     string      `json:"procID,omitempty"`
	MsgID      string      `json:"msgID,omitempty"`
	Structured []SDElement `json:"structured,omitempty"`
	Message    string      `json:"message,omitempty"`
}

// SDElement is one RFC 5424 structured-data element.
type SDElement struct {
	ID     string    `json:"id"`
	Params []SDParam `json:"params,omitempty"`
}

// SDParam is one RFC 5424 structured-data parameter.
type SDParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
