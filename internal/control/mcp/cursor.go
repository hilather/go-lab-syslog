package mcp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/hilather/go-lab-syslog/internal/domainerr"
)

type cursorCodec struct {
	key []byte
}

func newCursorCodec() *cursorCodec {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("mcp: cursor key: " + err.Error())
	}
	return &cursorCodec{key: key}
}

func (c *cursorCodec) encode(id string) string {
	if id == "" || c == nil {
		return ""
	}
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write([]byte(id))
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(id)) + "." + base64.RawURLEncoding.EncodeToString(sum)
}

func (c *cursorCodec) decode(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	idB64, sigB64, ok := strings.Cut(token, ".")
	if !ok || idB64 == "" || sigB64 == "" {
		return "", domainerr.New(domainerr.CursorStale, "cursor is not valid")
	}
	id, err := base64.RawURLEncoding.DecodeString(idB64)
	if err != nil {
		return "", domainerr.New(domainerr.CursorStale, "cursor is not valid")
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", domainerr.New(domainerr.CursorStale, "cursor is not valid")
	}
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write(id)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return "", domainerr.New(domainerr.CursorStale, "cursor is not valid")
	}
	return string(id), nil
}
