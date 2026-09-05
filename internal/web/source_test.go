package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestWebSourceHasNoXSSSinks(t *testing.T) {
	t.Parallel()
	root := webSrc(t)
	sinks := []*regexp.Regexp{
		regexp.MustCompile(`dangerouslySetInnerHTML`),
		regexp.MustCompile(`\binnerHTML\b`),
		regexp.MustCompile(`\bsrcdoc\b`),
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isWebSource(d.Name()) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		text := string(body)
		for _, re := range sinks {
			if re.MatchString(text) {
				t.Errorf("%s: forbidden XSS sink %s", rel, re.String())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWebSourceHasNoRelayControl(t *testing.T) {
	t.Parallel()
	root := webSrc(t)
	controls := []*regexp.Regexp{
		regexp.MustCompile(`label:\s*["']Forward["']`),
		regexp.MustCompile(`label:\s*["']Relay["']`),
		regexp.MustCompile(`>\s*(Forward|Relay)\s*<`),
		regexp.MustCompile(`name=["'](forward|relay)["']`),
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isWebSource(d.Name()) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		text := string(body)
		for _, re := range controls {
			if re.MatchString(text) {
				t.Errorf("%s: relay/forward control %s", rel, re.String())
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWebSourceDoesNotWriteTokensToStorage(t *testing.T) {
	t.Parallel()
	root := webSrc(t)
	bad := regexp.MustCompile(`(localStorage|sessionStorage)\.setItem\(`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isWebSource(d.Name()) {
			return nil
		}
		if strings.Contains(d.Name(), ".test.") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bad.Match(body) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s: must not write web storage", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func webSrc(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, "web", "src")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("web/src not found")
		}
		dir = parent
	}
}

func isWebSource(name string) bool {
	return strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")
}
