package signal

import (
	"bytes"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/4ba-Co/sentinel/internal/logger"
)

func init() {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
}

func TestIsPID1(t *testing.T) {
	if os.Getpid() == 1 {
		if !IsPID1() {
			t.Error("expected IsPID1=true when PID=1")
		}
	} else {
		if IsPID1() {
			t.Error("expected IsPID1=false when PID!=1")
		}
	}
}

func TestHandler(t *testing.T) {
	received := make(chan os.Signal, 1)

	h := NewHandler(func(sig os.Signal) {
		received <- sig
	})
	h.Start()
	defer h.Stop()

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	select {
	case sig := <-received:
		if sig != syscall.SIGUSR1 {
			t.Errorf("expected SIGUSR1, got %v", sig)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for signal")
	}
}

func TestHandlerDone(t *testing.T) {
	h := NewHandler(func(sig os.Signal) {})
	h.Start()
	defer h.Stop()

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)

	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Error("timeout waiting for Done channel")
	}
}

func TestHandlerStop(t *testing.T) {
	called := false
	h := NewHandler(func(sig os.Signal) {
		called = true
	})
	h.Start()
	h.Stop()

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(100 * time.Millisecond)

	if called {
		t.Error("handler should not be called after Stop")
	}
}
