package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

const healthcheckTimeout = 3 * time.Second

func cmdHealthcheck(args []string, stdout, stderr io.Writer) int {
	_ = stdout
	fs := flag.NewFlagSet("labsyslog healthcheck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "", "GET target (typically http://127.0.0.1:8088/v1/health/ready)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *url == "" {
		_, _ = fmt.Fprintln(stderr, "labsyslog healthcheck: --url is required")
		return 2
	}
	client := &http.Client{Timeout: healthcheckTimeout}
	resp, err := client.Get(*url)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "labsyslog healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		_, _ = fmt.Fprintf(stderr, "labsyslog healthcheck: %s\n", resp.Status)
		return 1
	}
	return 0
}
