# ADR 0001 — Use Go

**Status:** accepted  
**Date:** 2026-09-04

## Context

Sibling lab appliances are Go modules with scratch images and a shared agent methodology.

## Decision

LabSyslog is a first-party Go 1.26 module `github.com/hilather/go-lab-syslog`. Apache-2.0.

## Consequences

No rsyslog/syslog-ng wrapper. No Python collector. Shared CI/Make/docs patterns apply.
