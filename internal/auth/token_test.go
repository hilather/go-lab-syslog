package auth

import "testing"

func TestMinTokenBytes(t *testing.T) {
	if MinTokenBytes != 32 {
		t.Fatalf("MinTokenBytes = %d", MinTokenBytes)
	}
}

func TestEqualDigest(t *testing.T) {
	a := DigestSecret([]byte("0123456789abcdef0123456789abcdef"))
	b := DigestSecret([]byte("0123456789abcdef0123456789abcdef"))
	c := DigestSecret([]byte("ffffffffffffffffffffffffffffffff"))
	if !EqualDigest(a, b) {
		t.Fatal("equal digests")
	}
	if EqualDigest(a, c) {
		t.Fatal("different digests")
	}
}
