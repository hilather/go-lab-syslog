package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/control/mcp"
)

func cmdMCPStdio(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = stdout
	fs := flag.NewFlagSet("labsyslog mcp-stdio", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to a labsyslog.dev/v1alpha1 document")
	tokenFile := fs.String("token-file", "", "bearer token file (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		_, _ = fmt.Fprintln(stderr, "labsyslog mcp-stdio: --config is required")
		return 2
	}
	if *tokenFile == "" {
		_, _ = fmt.Fprintln(stderr, "labsyslog mcp-stdio: --token-file is required")
		return 2
	}

	svc, err := app.New(app.Config{
		BootstrapPath: *configPath,
		Compiler: compiler.Options{
			ConfigDir:        filepath.Dir(*configPath),
			ManagementListen: "off",
		},
	})
	if err != nil {
		return configErr(stderr, err)
	}
	defer func() { _ = svc.Close() }()
	if err := svc.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog mcp-stdio: %v\n", err)
		return 1
	}

	raw, err := os.ReadFile(*tokenFile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog mcp-stdio: token-file: %v\n", err)
		return 1
	}
	p, err := svc.Verifier().AuthenticateBearer(firstSecretLine(raw))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog mcp-stdio: token-file: %v\n", err)
		return 1
	}

	allowLegacy := false
	if snap := svc.Snapshot(); snap != nil {
		allowLegacy = snap.Document.Spec.Management.MCP.AllowLegacyClients
	}
	s, err := mcp.New(mcp.Config{
		Service:            svc,
		AllowLegacyClients: allowLegacy,
		FixedPrincipal:     &p,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog mcp-stdio: %v\n", err)
		return 1
	}
	if err := s.RunStdio(ctx); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog mcp-stdio: %v\n", err)
		return 1
	}
	return 0
}

func firstSecretLine(raw []byte) string {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		return s
	}
	return strings.TrimSpace(string(raw))
}
