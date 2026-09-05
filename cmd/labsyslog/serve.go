package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
)

func cmdServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = stdout // ready/listen lines are stderr until OBS-001
	fs := flag.NewFlagSet("labsyslog serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "path to a labsyslog.dev/v1alpha1 document")
	udpListen := fs.String("syslog-udp-listen", "", "UDP listen address (overrides spec.listeners.udp.address)")
	_ = fs.String("syslog-tcp-listen", "", "TCP listen address (TCP-001)")
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

	doc, err := config.LoadFile(*configPath)
	if err != nil {
		return configErr(stderr, err)
	}
	if err := compiler.Check(doc, filepath.Dir(*configPath)); err != nil {
		return configErr(stderr, err)
	}

	udpOn := doc.Spec.Listeners.UDP.Enabled == nil || *doc.Spec.Listeners.UDP.Enabled
	udpAddr := doc.Spec.Listeners.UDP.Address
	if *udpListen != "" {
		udpAddr = *udpListen
		udpOn = true
	}
	if !udpOn {
		_, _ = fmt.Fprintln(stderr, "labsyslog serve: UDP listener is disabled")
		return 1
	}

	mgmt := strings.TrimSpace(*mgmtListen)
	if mgmt == "" {
		mgmt = strings.TrimSpace(doc.Spec.Listeners.Management.Address)
	}
	mgmtOff := mgmt == "" || strings.EqualFold(mgmt, "off")

	srv, err := syslogserver.ListenUDP(ctx, syslogserver.Config{
		Addr:                udpAddr,
		UDPMaxDatagramBytes: int(doc.Spec.Syslog.UDPMaxDatagramBytes),
		MaxMessageBytes:     int(doc.Spec.Syslog.MaxMessageBytes),
		Parse:               syslogwire.OptionsFromParse(doc.Spec.Syslog.Parse),
		Behavior:            syslogserver.Behavior{Mode: syslogserver.BehaviorAccept},
		Handler:             syslogserver.NopHandler{},
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog serve: udp listen: %v\n", err)
		return 1
	}
	defer func() { _ = srv.Close() }()

	if *pidFile != "" {
		if err := os.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "labsyslog serve: pid-file: %v\n", err)
			return 1
		}
		defer func() { _ = os.Remove(*pidFile) }()
	}

	_, _ = fmt.Fprintln(stderr, "syslog udp listen "+srv.LocalAddr().String())
	if mgmtOff {
		_, _ = fmt.Fprintln(stderr, "management listen off")
	} else {
		_, _ = fmt.Fprintln(stderr, "management listen unbound")
	}

	<-ctx.Done()
	return 0
}
