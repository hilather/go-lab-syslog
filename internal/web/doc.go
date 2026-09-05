// Package web embeds the operator SPA. It is wired from cmd/labsyslog, not from rest.
//
// Production files are copied into dist/ by `make web-build`; go:embed cannot
// reach web/ because of web/go.mod.
package web
