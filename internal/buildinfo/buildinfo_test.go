package buildinfo

import (
	"strings"
	"sync"
	"testing"
)

func TestCurrentProtocols(t *testing.T) {
	info := Current()
	if info.Version == "" {
		t.Fatal("Version is empty")
	}
	if info.Commit == "" {
		t.Fatal("Commit is empty")
	}
	if info.BuildTime == "" {
		t.Fatal("BuildTime is empty")
	}
	if info.Protocols.ConfigAPI != ConfigAPIVersion {
		t.Fatalf("ConfigAPI = %q, want %q", info.Protocols.ConfigAPI, ConfigAPIVersion)
	}
	if info.Protocols.REST != RESTPrefix {
		t.Fatalf("REST = %q, want %q", info.Protocols.REST, RESTPrefix)
	}
	if info.Protocols.MCP != MCPProtocol {
		t.Fatalf("MCP = %q, want %q", info.Protocols.MCP, MCPProtocol)
	}
	s := info.String()
	if !strings.Contains(s, "labsyslog") || !strings.Contains(s, MCPProtocol) {
		t.Fatalf("String() = %q, missing expected tokens", s)
	}
	if info.Protocols.ConfigAPI != "labsyslog.dev/v1alpha1" {
		t.Fatalf("ConfigAPI = %q", info.Protocols.ConfigAPI)
	}
}

func TestInfoStringEmpty(t *testing.T) {
	var info Info
	if info.String() == "" {
		t.Fatal("empty Info.String() is empty")
	}
}

func TestCurrentConcurrent(t *testing.T) {
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			info := Current()
			if info.Version == "" || info.Commit == "" {
				errs <- "empty field"
			}
			if info.String() == "" {
				errs <- "empty String"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}
