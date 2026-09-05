// Command checkdocs verifies required root documents and markdown links.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// RequiredRootDocs are the documents this slice must find at the repository root.
var RequiredRootDocs = []string{
	"README.md",
	"AGENTS.md",
	"LICENSE",
	"CHANGELOG.md",
	"START-HERE.md",
	"SECURITY.md",
	"CONTRIBUTING.md",
	"Makefile",
	"Dockerfile",
	"go.mod",
	"docs/README.md",
	"docs/00-family-evaluation.md",
	"docs/01-architecture.md",
	"docs/02-syslog-semantics.md",
	"docs/03-message-store.md",
	"docs/04-state-and-configuration.md",
	"docs/05-control-plane-and-parity.md",
	"docs/06-rest-api.md",
	"docs/07-mcp-api.md",
	"docs/08-security-architecture.md",
	"docs/09-observability.md",
	"docs/10-testing-strategy.md",
	"docs/11-deployment.md",
	"docs/12-web-ui.md",
	"docs/13-integration-lab-swap.md",
	"docs/implementation-design.md",
	"docs/known-limitations.md",
	"docs/adr/README.md",
	"docs/adr/0001-use-go.md",
	"docs/adr/0002-in-tree-syslog-receive-only.md",
	"docs/adr/0003-ephemeral-state-and-gitops.md",
	"docs/adr/0004-shared-capability-registry.md",
	"docs/adr/0005-lab-static-bearer.md",
	"docs/adr/0006-pin-mcp-protocol-versions.md",
	"docs/adr/0007-rfc3164-and-5424-first-party.md",
	"docs/adr/0008-host-publish-514-residual.md",
	"docs/adr/0009-unmatched-filter-is-capture.md",
	"docs/adr/0010-container-514-net-bind-service.md",
	"docs/adr/0011-no-outbound-forward.md",
	"docs/adr/0012-tls-is-1-1.md",
	"tasks/00-program-board.md",
	"tasks/README.md",
	"tasks/wave-01-repository-foundation.md",
	".github/workflows/ci.yml",
}

// RequiredPhrases must appear in docs/ (NAT / userland-proxy).
var RequiredPhrases = []string{
	"NAT collision",
	"userland-proxy",
}

var mdLink = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "checkdocs: %v\n", err)
		os.Exit(1)
	}
	if err := Check(root); err != nil {
		fmt.Fprintf(os.Stderr, "checkdocs: %v\n", err)
		os.Exit(1)
	}
}

// Check verifies required documents exist and markdown internal links resolve.
func Check(root string) error {
	var missing []string
	for _, rel := range RequiredRootDocs {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required documents missing: %s", strings.Join(missing, ", "))
	}

	var broken []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "testdata" || base == "vendor" || base == "node_modules" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		prose := stripCode(body)
		for _, m := range mdLink.FindAllSubmatch(prose, -1) {
			target := strings.TrimSpace(string(m[1]))
			if i := strings.IndexAny(target, " \t"); i >= 0 {
				target = target[:i]
			}
			if skipLink(target) {
				continue
			}
			target = strings.SplitN(target, "#", 2)[0]
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(path), filepath.FromSlash(target))
			if _, err := os.Stat(resolved); err != nil {
				broken = append(broken, fmt.Sprintf("%s -> %s", rel, target))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(broken) > 0 {
		return fmt.Errorf("broken markdown links:\n  %s", strings.Join(broken, "\n  "))
	}
	return checkRequiredPhrases(root)
}

func checkRequiredPhrases(root string) error {
	docs := filepath.Join(root, "docs")
	var all []byte
	err := filepath.WalkDir(docs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		all = append(all, body...)
		all = append(all, '\n')
		return nil
	})
	if err != nil {
		return err
	}
	text := string(all)
	var missing []string
	for _, p := range RequiredPhrases {
		if !strings.Contains(text, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required documentation phrases missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

var (
	fencedCode = regexp.MustCompile("(?s)```.*?```")
	inlineCode = regexp.MustCompile("`[^`]*`")
)

func stripCode(body []byte) []byte {
	out := fencedCode.ReplaceAll(body, nil)
	return inlineCode.ReplaceAll(out, nil)
}

func skipLink(target string) bool {
	switch {
	case target == "":
		return true
	case strings.HasPrefix(target, "#"):
		return true
	case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"), strings.HasPrefix(target, "mailto:"):
		return true
	case strings.HasPrefix(target, "file:"):
		return true
	default:
		return false
	}
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}
