package syslogframing

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/hilather/go-lab-syslog/internal/testutil"
)

func FuzzNext(f *testing.F) {
	msg := "<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 - - hello"
	f.Add([]byte("57 "+msg+"57 "+msg), Auto)
	f.Add([]byte("<14>hello\n<15>world\n"), Auto)
	f.Add([]byte("<14>hello\n"), NonTransparent)
	f.Add([]byte("9 <14>hello"), OctetCounting)
	f.Add([]byte("<14>hello\x00<15>world\r\n"), NonTransparent)
	f.Add([]byte{}, Auto)
	f.Add([]byte("0 "), OctetCounting)
	f.Add([]byte("99999 x"), OctetCounting)
	for _, raw := range testutil.LoadCorpus(f, "syslogframing") {
		f.Add(raw, Auto)
		f.Add(raw, OctetCounting)
		f.Add(raw, NonTransparent)
	}

	f.Fuzz(func(t *testing.T, data []byte, mode string) {
		switch mode {
		case Auto, OctetCounting, NonTransparent, "":
		default:
			mode = Auto
		}
		s := NewScanner(bytes.NewReader(data), mode, 1024)
		for i := 0; i < 256; i++ {
			frame, err := s.Next()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, ErrFraming) || errors.Is(err, ErrOversize) {
					if errors.Is(err, ErrOversize) {
						continue
					}
					return
				}
				return
			}
			if len(frame) > 1024 {
				t.Fatalf("frame len %d exceeds cap", len(frame))
			}
		}
	})
}
