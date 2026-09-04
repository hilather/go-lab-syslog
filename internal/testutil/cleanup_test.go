package testutil

import "testing"

type closeRecorder struct {
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}

func TestCleanupHelpers(t *testing.T) {
	c := &closeRecorder{}
	t.Run("inner", func(t *testing.T) {
		MustClose(t, c)
		Cleanup(t, func() {})
		ctx := Context(t)
		if ctx.Err() != nil {
			t.Fatal("context already canceled")
		}
	})
	if !c.closed {
		t.Fatal("closer was not closed after subtest")
	}
}
