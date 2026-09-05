// Command labsyslog is the LabSyslog process entrypoint.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/hilather/go-lab-syslog/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[1] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current().String())
		return 0
	case "validate":
		return cmdValidate(args[2:], stdout, stderr)
	case "canonicalize":
		return cmdCanonicalize(args[2:], stdout, stderr)
	case "healthcheck":
		return cmdHealthcheck(args[2:], stdout, stderr)
	case "serve":
		return withShutdown(func(ctx context.Context) int {
			return cmdServe(ctx, args[2:], stdout, stderr)
		})
	case "mcp-stdio":
		return withShutdown(func(ctx context.Context) int {
			return cmdMCPStdio(ctx, args[2:], stdout, stderr)
		})
	case "send":
		_, _ = fmt.Fprintln(stderr, "labsyslog send is forbidden")
		return 1
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		printUsage(stderr)
		return 2
	}
}

func withShutdown(fn func(context.Context) int) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return fn(ctx)
}

func notImplemented(cmd string, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stderr, "labsyslog %s: not implemented\n", cmd)
	return 1
}

func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, usageText)
}

const usageText = `usage: labsyslog <command>

LabSyslog is a receive-only syslog lab appliance.
validate and canonicalize load a fail-closed labsyslog.dev/v1alpha1
document. serve binds UDP/TCP syslog. Management REST /v1, MCP /mcp,
and the operator SPA at / bind only when --management-listen is an
address (or spec.listeners.management.address). --management-listen=off
is first-class. There is no send command.

Commands:
  version         print build and protocol metadata
  help            print this help
  validate        fail-closed YAML check (--config)
  canonicalize    emit canonical spec (--config)
  serve           bind syslog (--config, --syslog-udp-listen,
                  --syslog-tcp-listen, --management-listen,
                  --shutdown-timeout, --pid-file)
  healthcheck     probe GET /v1/health/ready (--url)
  mcp-stdio       Streamable MCP over stdio (--config, --token-file)
`
