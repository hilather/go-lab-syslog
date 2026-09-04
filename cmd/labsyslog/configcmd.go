package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hilather/go-lab-syslog/internal/compiler"
	"github.com/hilather/go-lab-syslog/internal/config"
	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	path, code := parseConfigFlag("validate", args, stderr)
	if code != 0 {
		return code
	}
	_, rev, err := loadCanonical(path)
	if err != nil {
		return configErr(stderr, err)
	}
	_, _ = fmt.Fprintln(stdout, "revision: "+rev)
	return 0
}

func cmdCanonicalize(args []string, stdout, stderr io.Writer) int {
	path, code := parseConfigFlag("canonicalize", args, stderr)
	if code != 0 {
		return code
	}
	canon, rev, err := loadCanonical(path)
	if err != nil {
		return configErr(stderr, err)
	}
	if _, err := stdout.Write(canon); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintln(stderr, "revision: "+rev)
	return 0
}

func loadCanonical(path string) ([]byte, string, error) {
	doc, err := config.LoadFile(path)
	if err != nil {
		return nil, "", err
	}
	if err := compiler.Check(doc, filepath.Dir(path)); err != nil {
		return nil, "", err
	}
	canon, err := config.EncodeCanonical(doc)
	if err != nil {
		return nil, "", err
	}
	return canon, compiler.Revision(canon), nil
}

func parseConfigFlag(cmd string, args []string, stderr io.Writer) (string, int) {
	fs := flag.NewFlagSet("labsyslog "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", "", "path to a labsyslog.dev/v1alpha1 document")
	if err := fs.Parse(args); err != nil {
		return "", 2
	}
	if *path == "" {
		_, _ = fmt.Fprintf(stderr, "labsyslog %s: --config is required\n", cmd)
		return "", 2
	}
	return *path, 0
}

func configErr(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintln(stderr, err)
	if _, ok := domainerr.As(err); ok {
		return 2
	}
	if os.IsNotExist(err) {
		return 1
	}
	return 1
}
