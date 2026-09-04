package auth

import "testing"

func TestMinTokenBytes(t *testing.T) {
	if MinTokenBytes != 32 {
		t.Fatalf("MinTokenBytes = %d", MinTokenBytes)
	}
}
