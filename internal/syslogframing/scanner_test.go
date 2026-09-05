package syslogframing

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

const docsOctetMSG = "<165>1 2026-09-04T20:52:35.000Z sut-1 sshd 1234 - - hello"

func TestOctetTwoBackToBack(t *testing.T) {
	msg := docsOctetMSG
	input := octetFrame(msg) + octetFrame(msg)
	s := NewScanner(strings.NewReader(input), OctetCounting, 0)
	got := readAll(t, s)
	if len(got) != 2 || got[0] != msg || got[1] != msg {
		t.Fatalf("frames = %#v", got)
	}
}

func TestNonTransparentTwoNL(t *testing.T) {
	s := NewScanner(strings.NewReader("<14>hello\n<15>world\n"), NonTransparent, 0)
	got := readAll(t, s)
	if len(got) != 2 || got[0] != "<14>hello" || got[1] != "<15>world" {
		t.Fatalf("frames = %#v", got)
	}
}

func TestNonTransparentNULAndCRLF(t *testing.T) {
	s := NewScanner(bytes.NewReader([]byte("<14>hello\x00<15>world\r\n")), NonTransparent, 0)
	got := readAll(t, s)
	if len(got) != 2 || got[0] != "<14>hello" || got[1] != "<15>world" {
		t.Fatalf("frames = %#v", got)
	}
}

func TestAutoDigitSPIsOctet(t *testing.T) {
	msg := "<14>hello"
	s := NewScanner(strings.NewReader(octetFrame(msg)+octetFrame(msg)), Auto, 0)
	got := readAll(t, s)
	if s.Mode() != OctetCounting {
		t.Fatalf("mode = %q", s.Mode())
	}
	if len(got) != 2 || got[0] != msg || got[1] != msg {
		t.Fatalf("frames = %#v", got)
	}
}

func TestAutoPRIIsNonTransparent(t *testing.T) {
	s := NewScanner(strings.NewReader("<14>hello\n<15>world\n"), Auto, 0)
	got := readAll(t, s)
	if s.Mode() != NonTransparent {
		t.Fatalf("mode = %q, PRI < must not select octet-counting", s.Mode())
	}
	if len(got) != 2 {
		t.Fatalf("frames = %#v", got)
	}
}

func TestAutoCannotFlipAfterPRI(t *testing.T) {
	s := NewScanner(strings.NewReader("<14>hello\n9 <14>hello"), Auto, 0)
	frame, err := s.Next()
	if err != nil || string(frame) != "<14>hello" {
		t.Fatalf("first frame = %q err=%v", frame, err)
	}
	if s.Mode() != NonTransparent {
		t.Fatalf("mode = %q", s.Mode())
	}
	_, err = s.Next()
	if !errors.Is(err, ErrFraming) {
		t.Fatalf("disagree err = %v, want ErrFraming", err)
	}
}

func TestAutoCannotFlipAfterOctet(t *testing.T) {
	s := NewScanner(strings.NewReader(octetFrame("<14>hello")+"<14>hello\n"), Auto, 0)
	frame, err := s.Next()
	if err != nil || string(frame) != "<14>hello" {
		t.Fatalf("first = %q err=%v", frame, err)
	}
	_, err = s.Next()
	if !errors.Is(err, ErrFraming) {
		t.Fatalf("disagree err = %v, want ErrFraming", err)
	}
}

func TestOctetOversizeNoPartial(t *testing.T) {
	s := NewScanner(strings.NewReader("20 <14>this-is-too-long"), OctetCounting, 8)
	frame, err := s.Next()
	if !errors.Is(err, ErrFraming) {
		t.Fatalf("err = %v, want ErrFraming", err)
	}
	if frame != nil {
		t.Fatalf("partial frame %q", frame)
	}
}

func TestOctetSingleDigitOversize(t *testing.T) {
	s := NewScanner(strings.NewReader("9 <14>hello"), OctetCounting, 8)
	frame, err := s.Next()
	if !errors.Is(err, ErrFraming) {
		t.Fatalf("err = %v, want ErrFraming", err)
	}
	if frame != nil {
		t.Fatalf("partial frame %q", frame)
	}
}

func TestOctetLeadingZeroFramingError(t *testing.T) {
	s := NewScanner(strings.NewReader("09 <14>hello"), OctetCounting, 64)
	if _, err := s.Next(); !errors.Is(err, ErrFraming) {
		t.Fatalf("err = %v", err)
	}
}

func TestNonTransparentOversizeContinues(t *testing.T) {
	s := NewScanner(strings.NewReader("123456789\n<14>ok\n"), NonTransparent, 8)
	_, err := s.Next()
	if !errors.Is(err, ErrOversize) {
		t.Fatalf("err = %v, want ErrOversize", err)
	}
	frame, err := s.Next()
	if err != nil || string(frame) != "<14>ok" {
		t.Fatalf("frame = %q err=%v", frame, err)
	}
}

func TestNonTransparentExactMaxCRLF(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 8)
	input := append(append(payload, '\r', '\n'), []byte("<14>ok\n")...)
	s := NewScanner(bytes.NewReader(input), NonTransparent, 8)
	frame, err := s.Next()
	if err != nil || !bytes.Equal(frame, payload) {
		t.Fatalf("exact max + CRLF: frame = %q err=%v", frame, err)
	}
	frame, err = s.Next()
	if err != nil || string(frame) != "<14>ok" {
		t.Fatalf("follow-on = %q err=%v", frame, err)
	}
}

func TestNonTransparentMaxPlusOneCRLF(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 9)
	input := append(append(payload, '\r', '\n'), []byte("<14>ok\n")...)
	s := NewScanner(bytes.NewReader(input), NonTransparent, 8)
	if _, err := s.Next(); !errors.Is(err, ErrOversize) {
		t.Fatalf("err = %v, want ErrOversize", err)
	}
	frame, err := s.Next()
	if err != nil || string(frame) != "<14>ok" {
		t.Fatalf("session must continue: frame = %q err=%v", frame, err)
	}
}

func TestAutoOversizeThenDigitSPDisagrees(t *testing.T) {
	s := NewScanner(strings.NewReader("AAAAAAAAA\n9 <14>hello\n"), Auto, 8)
	if _, err := s.Next(); !errors.Is(err, ErrOversize) {
		t.Fatalf("first = %v, want ErrOversize", err)
	}
	if s.Mode() != NonTransparent {
		t.Fatalf("mode = %q after oversize", s.Mode())
	}
	frame, err := s.Next()
	if !errors.Is(err, ErrFraming) {
		t.Fatalf("disagree after oversize: frame=%q err=%v", frame, err)
	}
}

func TestEmptyNonTransparentFrame(t *testing.T) {
	s := NewScanner(strings.NewReader("\n<14>ok\n"), NonTransparent, 0)
	frame, err := s.Next()
	if err != nil || len(frame) != 0 {
		t.Fatalf("empty frame = %q err=%v", frame, err)
	}
	frame, err = s.Next()
	if err != nil || string(frame) != "<14>ok" {
		t.Fatalf("frame = %q err=%v", frame, err)
	}
}

func TestCRNotStrippedBeforeNUL(t *testing.T) {
	s := NewScanner(bytes.NewReader([]byte("hello\r\x00")), NonTransparent, 0)
	frame, err := s.Next()
	if err != nil || string(frame) != "hello\r" {
		t.Fatalf("frame = %q err=%v", frame, err)
	}
}

func TestCRBeforeNULAtExactMaxIsContent(t *testing.T) {
	s := NewScanner(bytes.NewReader([]byte("hello\r\x00")), NonTransparent, 6)
	frame, err := s.Next()
	if err != nil || string(frame) != "hello\r" {
		t.Fatalf("CR-before-NUL at max: frame = %q err=%v", frame, err)
	}
}

func TestIncompleteOctetIsFraming(t *testing.T) {
	s := NewScanner(strings.NewReader("9 <14>hel"), OctetCounting, 64)
	if _, err := s.Next(); !errors.Is(err, ErrFraming) {
		t.Fatalf("err = %v", err)
	}
}

func octetFrame(msg string) string {
	return itoa(len(msg)) + " " + msg
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func readAll(t *testing.T, s *Scanner) []string {
	t.Helper()
	var out []string
	for {
		frame, err := s.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("Next: %v after %v", err, out)
		}
		out = append(out, string(frame))
	}
}
