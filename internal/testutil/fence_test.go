package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
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

var adapterPkgs = []string{
	"internal/control/rest",
	"internal/control/mcp",
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

	assertNoUnlistedNested(t, root, fencePkgList())
}

func TestNoDialIdentifiers(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	for _, rel := range noDialPkgs {
		walkProductionGo(t, root, rel, func(fileRel string) {
			f, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(fileRel)), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", fileRel, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if forbiddenIdentName(id.Name) {
					t.Errorf("%s: forbidden identifier %s", fileRel, id.Name)
				}
				return true
			})
		})
	}
}

func TestForbiddenIdentSnippets(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "Dial",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.Dial(\"udp\", \"\") }\n",
			want: []string{"Dial"},
		},
		{
			name: "DialTimeout",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.DialTimeout(\"tcp\", \"\", 0) }\n",
			want: []string{"DialTimeout"},
		},
		{
			name: "Dialer",
			src:  "package p\nimport \"net\"\nvar _ = net.Dialer{}\n",
			want: []string{"Dialer"},
		},
		{
			name: "DialContext",
			src:  "package p\nimport (\n\"context\"\n\"net\"\n)\nfunc f() { var d net.Dialer; _, _ = d.DialContext(context.Background(), \"udp\", \"\") }\n",
			want: []string{"DialContext", "Dialer"},
		},
		{
			name: "DialUDP",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.DialUDP(\"udp\", nil, nil) }\n",
			want: []string{"DialUDP"},
		},
		{
			name: "DialTCP",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.DialTCP(\"tcp\", nil, nil) }\n",
			want: []string{"DialTCP"},
		},
		{
			name: "DialIP",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.DialIP(\"ip4:icmp\", nil, nil) }\n",
			want: []string{"DialIP"},
		},
		{
			name: "DialUnix",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.DialUnix(\"unix\", nil, nil) }\n",
			want: []string{"DialUnix"},
		},
		{
			name: "NewRequest",
			src:  "package p\nimport \"net/http\"\nfunc f() { _, _ = http.NewRequest(\"GET\", \"/\", nil) }\n",
			want: []string{"NewRequest"},
		},
		{
			name: "NewRequestWithContext",
			src:  "package p\nimport (\n\"context\"\n\"net/http\"\n)\nfunc f() { _, _ = http.NewRequestWithContext(context.Background(), \"GET\", \"/\", nil) }\n",
			want: []string{"NewRequestWithContext"},
		},
		{
			name: "ListenPacket allowed",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.ListenPacket(\"udp\", \":0\") }\n",
		},
		{
			name: "Listen allowed",
			src:  "package p\nimport \"net\"\nfunc f() { _, _ = net.Listen(\"tcp\", \":0\") }\n",
		},
		{
			name: "Accept allowed",
			src:  "package p\nimport \"net\"\nfunc f(l net.Listener) { _, _ = l.Accept() }\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := forbiddenIdentsInSource(t, tt.src)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("forbidden idents = %q, want %q", got, tt.want)
			}
		})
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
			if !isProductionGo(d.Name()) {
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
	var out []string
	walkProductionGo(t, root, rel, func(fileRel string) {
		out = append(out, fileImports(t, filepath.Join(root, filepath.FromSlash(fileRel)))...)
	})
	return out
}

func walkProductionGo(t *testing.T, root, rel string, fn func(fileRel string)) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("missing package %s", rel)
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !isProductionGo(d.Name()) {
			return nil
		}
		fileRel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		fn(filepath.ToSlash(fileRel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoUnlistedNested(t *testing.T, root string, listed []string) {
	t.Helper()
	set := map[string]bool{}
	for _, p := range listed {
		set[p] = true
	}
	for _, rel := range listed {
		walkProductionGo(t, root, rel, func(fileRel string) {
			pkg := path.Dir(fileRel)
			if !set[pkg] {
				t.Errorf("%s: nested package %s is not on the fence list", fileRel, pkg)
			}
		})
	}
}

func fencePkgList() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range dataPlanePkgs {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range noDialPkgs {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range adapterPkgs {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
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

func forbiddenIdentName(name string) bool {
	return strings.HasPrefix(name, "Dial") || strings.HasPrefix(name, "NewRequest")
}

func forbiddenIdentsInSource(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "snippet.go", src, 0)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if !forbiddenIdentName(id.Name) || seen[id.Name] {
			return true
		}
		seen[id.Name] = true
		out = append(out, id.Name)
		return true
	})
	sort.Strings(out)
	return out
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
