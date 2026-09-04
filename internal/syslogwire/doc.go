// Package syslogwire is the first-party RFC 3164 and RFC 5424 codec.
// It must not import syslog libraries. Production code must not Dial.
//
// Parse returns model.Parsed plus a parseWarning token. Serialize is for
// tests and export, not forwarding.
package syslogwire
