package runner

import (
	"bytes"
	"testing"
	"time"
)

func TestActivityWriter(t *testing.T) {
	var buf bytes.Buffer
	aw := NewActivityWriter(&buf)

	initialIdle := aw.IdleDuration()
	if initialIdle > time.Second {
		t.Errorf("expected initial idle < 1s, got %v", initialIdle)
	}

	time.Sleep(50 * time.Millisecond)

	idleBeforeWrite := aw.IdleDuration()
	if idleBeforeWrite < 50*time.Millisecond {
		t.Errorf("expected idle >= 50ms, got %v", idleBeforeWrite)
	}

	n, err := aw.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected n=4, got %d", n)
	}

	idleAfterWrite := aw.IdleDuration()
	if idleAfterWrite > 10*time.Millisecond {
		t.Errorf("expected idle < 10ms after write, got %v", idleAfterWrite)
	}

	if buf.String() != "test" {
		t.Errorf("expected buffer='test', got %s", buf.String())
	}
}

func TestActivityWriterLastActive(t *testing.T) {
	var buf bytes.Buffer
	aw := NewActivityWriter(&buf)

	before := time.Now()
	aw.Write([]byte("x"))
	after := time.Now()

	lastActive := aw.LastActive()
	if lastActive.Before(before) || lastActive.After(after) {
		t.Errorf("LastActive %v not between %v and %v", lastActive, before, after)
	}
}
