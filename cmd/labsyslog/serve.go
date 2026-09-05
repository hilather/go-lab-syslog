package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/compiler"
)

func cmdServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = stdout // ready/listen lines are stderr until OBS-001
	fs := flag.NewFlagSet("labsyslog serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to a labsyslog.dev/v1alpha1 document")
	udpListen := fs.String("syslog-udp-listen", "", "UDP listen address (overrides spec.listeners.udp.address)")
	tcpListen := fs.String("syslog-tcp-listen", "", "TCP listen address (overrides spec.listeners.tcp.address)")
	mgmtListen := fs.String("management-listen", "", "management listen address or off")
	shutdownTimeout := fs.String("shutdown-timeout", "5s", "drain timeout on SIGTERM")
	pidFile := fs.String("pid-file", "", "write process id then remove on exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *configPath == "" {
		_, _ = fmt.Fprintln(stderr, "labsyslog serve: --config is required")
		return 2
	}
	if _, err := time.ParseDuration(*shutdownTimeout); err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog serve: --shutdown-timeout: %v\n", err)
		return 2
	}

	svc, err := app.New(app.Config{
		BootstrapPath: *configPath,
		Compiler: compiler.Options{
			ConfigDir:        filepath.Dir(*configPath),
			UDPListen:        *udpListen,
			TCPListen:        *tcpListen,
			ManagementListen: *mgmtListen,
		},
	})
	if err != nil {
		return configErr(stderr, err)
	}
	if err := svc.Start(ctx); err != nil {
		_ = svc.Close()
		_, _ = fmt.Fprintf(stderr, "labsyslog serve: %v\n", err)
		return 1
	}
	defer func() { _ = svc.Close() }()

	if addr := svc.UDPAddr(); addr != nil {
		_, _ = fmt.Fprintln(stderr, "syslog udp listen "+addr.String())
	}
	if addr := svc.TCPAddr(); addr != nil {
		_, _ = fmt.Fprintln(stderr, "syslog tcp listen "+addr.String())
	}
	if addr := svc.ManagementAddr(); addr != "" {
		_, _ = fmt.Fprintln(stderr, "management listen "+addr)
	} else {
		_, _ = fmt.Fprintln(stderr, "management listen off")
	}

	if *pidFile != "" {
		if err := os.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "labsyslog serve: pid-file: %v\n", err)
			return 1
		}
		defer func() { _ = os.Remove(*pidFile) }()
	}

	<-ctx.Done()
	return 0
}
