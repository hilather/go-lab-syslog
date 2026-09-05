package syslogframing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goldenFile struct {
	Comment         string   `json:"comment"`
	Mode            string   `json:"mode"`
	MaxMessageBytes int      `json:"maxMessageBytes"`
	Input           string   `json:"input"`
	InputBase64     string   `json:"inputBase64"`
	Frames          []string `json:"frames"`
	Error           string   `json:"error"`
	Sticky          string   `json:"sticky"`
}

func TestGoldenFraming(t *testing.T) {
	root := framingRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no golden files under testdata/framing")
	}
	for _, path := range files {
		rel, _ := filepath.Rel(root, path)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			runGolden(t, path)
		})
	}
}

func runGolden(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw := []byte(g.Input)
	if g.InputBase64 != "" {
		raw, err = base64.StdEncoding.DecodeString(g.InputBase64)
		if err != nil {
			t.Fatalf("inputBase64: %v", err)
		}
	}
	s := NewScanner(bytes.NewReader(raw), g.Mode, g.MaxMessageBytes)
	var got []string
	var gotErr error
	for {
		frame, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			gotErr = err
			break
		}
		got = append(got, string(frame))
	}
	if len(got) != len(g.Frames) {
		t.Fatalf("frames = %#v, want %#v (err=%v)", got, g.Frames, gotErr)
	}
	for i := range g.Frames {
		if got[i] != g.Frames[i] {
			t.Fatalf("frames[%d] = %q, want %q", i, got[i], g.Frames[i])
		}
	}
	switch g.Error {
	case "":
		if gotErr != nil {
			t.Fatalf("err = %v, want nil", gotErr)
		}
	case "framing":
		if !errors.Is(gotErr, ErrFraming) {
			t.Fatalf("err = %v, want ErrFraming", gotErr)
		}
	case "oversize":
		if !errors.Is(gotErr, ErrOversize) {
			t.Fatalf("err = %v, want ErrOversize", gotErr)
		}
	default:
		t.Fatalf("unknown golden error %q", g.Error)
	}
	if g.Sticky != "" && s.Mode() != g.Sticky {
		t.Fatalf("sticky = %q, want %q", s.Mode(), g.Sticky)
	}
}

func framingRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		p := filepath.Join(dir, "testdata", "framing")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("testdata/framing not found")
		}
		dir = parent
	}
}
