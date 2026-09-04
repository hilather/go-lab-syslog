package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

const modulePath = "github.com/hilather/go-lab-syslog"

var dataPlanePkgs = []string{
	"internal/syslogwire",
	"internal/syslogframing",
	"internal/syslogserver",
	"internal/store",
}

var noDialPkgs = []string{
	"internal/syslogserver",
	"internal/store",
	"internal/app",
	"internal/syslogwire",
	"internal/syslogframing",
}

var forbiddenIdent = map[string]bool{
	"Dial":        true,
	"DialTimeout": true,
	"Dialer":      true,
	"NewRequest":  true,
}

var forbiddenModules = []string{
	"log/syslog",
	"gopkg.in/mcuadros/go-syslog",
	"github.com/leodido/go-syslog",
	"github.com/prometheus/",
}

func TestImportFence(t *testing.T) {
	root := moduleRoot(t)
	for _, rel := range dataPlanePkgs {
		for _, imp := range productionImports(t, root, rel) {
			if strings.Contains(imp, "/internal/control") || strings.HasSuffix(imp, "/internal/control") {
				t.Errorf("%s imports control plane %s", rel, imp)
			}
			if strings.Contains(imp, "/internal/web") || strings.HasSuffix(imp, "/internal/web") {
				t.Errorf("%s imports web %s", rel, imp)
			}
			if imp == "net/http" {
				t.Errorf("%s imports net/http", rel)
			}
		}
	}

	restImps := productionImports(t, root, "internal/control/rest")
	mcpImps := productionImports(t, root, "internal/control/mcp")
	for _, imp := range restImps {
		if strings.Contains(imp, "/internal/web") || strings.HasSuffix(imp, "/internal/web") {
			t.Errorf("internal/control/rest imports web %s", imp)
		}
		if strings.Contains(imp, "/internal/control/mcp") {
			t.Errorf("internal/control/rest imports mcp %s", imp)
		}
	}
	for _, imp := range mcpImps {
		if strings.Contains(imp, "/internal/control/rest") {
			t.Errorf("internal/control/mcp imports rest %s", imp)
		}
	}

	scanForbidden := func(base string) {
		err := filepath.WalkDir(filepath.Join(root, base), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "testdata", "syslogtest", "testutil":
					return fs.SkipDir
				}
				return nil
			}
			if !isProductionGo(d.Name()) {
				return nil
			}
			rel, _ := filepath.Rel(root, p)
			for _, imp := range fileImports(t, p) {
				if forbiddenImport(imp) {
					t.Errorf("%s imports forbidden module %s", rel, imp)
				}
				if strings.Contains(imp, "/internal/syslogtest") {
					t.Errorf("%s imports syslogtest %s", rel, imp)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	scanForbidden("internal")
	scanForbidden("cmd")
}

func TestNoDialIdentifiers(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	for _, rel := range noDialPkgs {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		ents, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("missing package %s", rel)
				continue
			}
			t.Fatal(err)
		}
		for _, e := range ents {
			if e.IsDir() || !isProductionGo(e.Name()) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if forbiddenIdent[id.Name] {
					t.Errorf("%s/%s: forbidden identifier %s", rel, e.Name(), id.Name)
				}
				return true
			})
		}
	}
}

func TestNoImportCycles(t *testing.T) {
	root := moduleRoot(t)
	graph := map[string][]string{}
	scan := func(base string) {
		err := filepath.WalkDir(filepath.Join(root, base), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			relDir, err := filepath.Rel(root, filepath.Dir(p))
			if err != nil {
				return err
			}
			pkg := path.Clean(filepath.ToSlash(relDir))
			for _, imp := range fileImports(t, p) {
				if !strings.HasPrefix(imp, modulePath+"/") {
					continue
				}
				local := strings.TrimPrefix(imp, modulePath+"/")
				graph[pkg] = append(graph[pkg], local)
			}
			if _, ok := graph[pkg]; !ok {
				graph[pkg] = nil
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	scan("internal")
	scan("cmd")

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var dfs func(string) bool
	dfs = func(n string) bool {
		color[n] = gray
		stack = append(stack, n)
		for _, m := range graph[n] {
			switch color[m] {
			case white:
				if dfs(m) {
					return true
				}
			case gray:
				t.Errorf("import cycle: %s", strings.Join(append(stack, m), " -> "))
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return false
	}
	for n := range graph {
		if color[n] == white {
			if dfs(n) {
				return
			}
		}
	}
}

func productionImports(t *testing.T, root, rel string) []string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("missing package %s", rel)
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || !isProductionGo(e.Name()) {
			continue
		}
		out = append(out, fileImports(t, filepath.Join(dir, e.Name()))...)
	}
	return out
}

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out
}

func isProductionGo(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func forbiddenImport(imp string) bool {
	for _, bad := range forbiddenModules {
		if strings.HasSuffix(bad, "/") {
			if strings.HasPrefix(imp, bad) {
				return true
			}
			continue
		}
		if imp == bad || strings.HasPrefix(imp, bad+"/") {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
