// Package syslogserver listens for syslog and runs the ingest pipeline.
// Production code must Listen/Accept only; it must not Dial.
//
// Binding order is size/framing → admission → behavior.mode → parse →
// classify → Handler.Insert. Parse runs inside the pipeline via
// syslogwire; UDP/TCP call sites must not parse first.
package syslogserver
