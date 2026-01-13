package runner

import (
	"io"
	"sync"
	"time"
)

type ActivityWriter struct {
	w          io.Writer
	mu         sync.Mutex
	lastActive time.Time
}

func NewActivityWriter(w io.Writer) *ActivityWriter {
	return &ActivityWriter{
		w:          w,
		lastActive: time.Now(),
	}
}

func (a *ActivityWriter) Write(p []byte) (n int, err error) {
	a.mu.Lock()
	a.lastActive = time.Now()
	a.mu.Unlock()
	return a.w.Write(p)
}

func (a *ActivityWriter) LastActive() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastActive
}

func (a *ActivityWriter) IdleDuration() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActive)
}
