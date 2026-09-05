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
	"github.com/hilather/go-lab-syslog/internal/model"
	"github.com/hilather/go-lab-syslog/internal/store"
	"github.com/hilather/go-lab-syslog/internal/syslogserver"
	"github.com/hilather/go-lab-syslog/internal/syslogwire"
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

func ingestFromDoc(doc *model.Document) (syslogserver.Config, *store.Store, error) {
	if doc == nil {
		return syslogserver.Config{}, nil, fmt.Errorf("document is empty")
	}
	admit, err := syslogserver.NewCIDRAdmission(doc.Spec.Admission)
	if err != nil {
		return syslogserver.Config{}, nil, err
	}
	class, err := syslogserver.NewFilterClassifier(doc.Spec.Filters)
	if err != nil {
		return syslogserver.Config{}, nil, err
	}
	st := store.NewFromSpec(doc.Spec.Store)
	cfg := syslogserver.Config{
		UDPMaxDatagramBytes: int(doc.Spec.Syslog.UDPMaxDatagramBytes),
		MaxMessageBytes:     int(doc.Spec.Syslog.MaxMessageBytes),
		Framing:             doc.Spec.Listeners.TCP.Framing,
		TCPIdleTimeout:      doc.Spec.Syslog.TCPIdleTimeout.Duration(),
		SessionTimeout:      doc.Spec.Admission.SessionTimeout.Duration(),
		MaxTCPConns:         doc.Spec.Admission.MaxTCPConns,
		MaxTCPConnsPerIP:    doc.Spec.Admission.MaxTCPConnsPerIP,
		Parse:               syslogwire.OptionsFromParse(doc.Spec.Syslog.Parse),
		Admission:           admit,
		Classifier:          class,
		Behavior:            syslogserver.Behavior{Mode: doc.Spec.Syslog.Behavior.Mode},
		Handler: syslogserver.HandlerFunc(func(_ context.Context, msg model.Message) error {
			_, err := st.Insert(msg)
			return err
		}),
		Metrics: &syslogserver.Metrics{},
	}
	return cfg, st, nil
}
