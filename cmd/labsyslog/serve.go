package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hilather/go-lab-syslog/internal/app"
	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/control/mcp"
	"github.com/hilather/go-lab-syslog/internal/control/rest"
	"github.com/hilather/go-lab-syslog/internal/observability"
	"github.com/hilather/go-lab-syslog/internal/web"
)

func cmdServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	observability.SetDefaultJSON(stdout)
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
	drain, err := time.ParseDuration(*shutdownTimeout)
	if err != nil {
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
	if snap := svc.Snapshot(); snap != nil {
		observability.SetLevel(snap.Document.Spec.Observability.LogLevel)
	}
	if err := svc.Start(ctx); err != nil {
		_ = svc.Close()
		_, _ = fmt.Fprintf(stderr, "labsyslog serve: %v\n", err)
		return 1
	}
	defer func() { _ = svc.Close() }()

	httpSrv, mcpSrv, err := mountManagement(svc)
	if err != nil {
		_ = svc.Close()
		_, _ = fmt.Fprintf(stderr, "labsyslog serve: mcp: %v\n", err)
		return 1
	}
	if mcpSrv != nil {
		defer mcpSrv.Close()
	}
	if httpSrv != nil {
		go func() {
			ln := svc.ManagementListener()
			if ln == nil {
				return
			}
			if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
				_, _ = fmt.Fprintf(stderr, "labsyslog serve: rest: %v\n", err)
			}
		}()
		defer func() {
			shCtx, cancel := context.WithTimeout(context.Background(), drain)
			defer cancel()
			_ = httpSrv.Shutdown(shCtx)
		}()
	}

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

func mountManagement(svc *app.Service) (*http.Server, *mcp.Server, error) {
	if svc == nil || svc.ManagementListener() == nil {
		return nil, nil, nil
	}
	mcpSrv, err := mcp.New(mcp.Config{Service: svc})
	if err != nil {
		return nil, nil, err
	}
	path := mcp.DefaultPath
	if snap := svc.Snapshot(); snap != nil && snap.Document.Spec.Listeners.Management.MCPPath != "" {
		path = snap.Document.Spec.Listeners.Management.MCPPath
	}
	restSrv := rest.Mount(svc, rest.Options{
		UI:        web.NewHandler(nil),
		UIEnabled: func() bool { return uiEnabled(svc) },
	})
	mux := http.NewServeMux()
	mux.Handle(path, mcpSrv.Handler())
	if restSrv != nil && restSrv.Handler != nil {
		mux.Handle("/", restSrv.Handler)
	}
	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}, mcpSrv, nil
}

func uiEnabled(svc *app.Service) bool {
	if svc == nil {
		return false
	}
	snap := svc.Snapshot()
	if snap == nil {
		return false
	}
	if snap.Document.Spec.UI.Enabled == nil {
		return true
	}
	return *snap.Document.Spec.UI.Enabled
}
