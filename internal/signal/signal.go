package signal

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/4ba-Co/sentinel/internal/logger"
)

type Handler struct {
	sigCh    chan os.Signal
	stopCh   chan struct{}
	doneCh   chan struct{}
	onSignal func(os.Signal)
}

func NewHandler(onSignal func(os.Signal)) *Handler {
	return &Handler{
		sigCh:    make(chan os.Signal, 1),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		onSignal: onSignal,
	}
}

func (h *Handler) Start() {
	signal.Notify(h.sigCh,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGHUP,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)

	go h.loop()
}

func (h *Handler) loop() {
	for {
		select {
		case sig := <-h.sigCh:
			logger.Info("received signal: %v", sig)
			if h.onSignal != nil {
				h.onSignal(sig)
			}
			close(h.doneCh)
		case <-h.stopCh:
			return
		}
	}
}

func (h *Handler) Stop() {
	signal.Stop(h.sigCh)
	close(h.stopCh)
}

func (h *Handler) Done() <-chan struct{} {
	return h.doneCh
}

func IsPID1() bool {
	return os.Getpid() == 1
}

// ReapResult holds the exit status collected by the zombie reaper
// for a tracked child process.
type ReapResult struct {
	ExitStatus int
}

// trackedPIDs maps child PIDs to channels that receive their exit status
// when the zombie reaper collects them. This prevents the reaper from
// silently consuming the main child's exit status before runner.Wait()
// can collect it.
var trackedPIDs sync.Map // int -> chan ReapResult

// TrackPID registers a PID so the reaper will forward its exit status
// instead of silently discarding it. Returns a channel that receives
// the result if the reaper collects this PID before cmd.Wait() does.
func TrackPID(pid int) <-chan ReapResult {
	ch := make(chan ReapResult, 1)
	trackedPIDs.Store(pid, ch)
	return ch
}

// UntrackPID removes a PID from reaper tracking.
func UntrackPID(pid int) {
	trackedPIDs.Delete(pid)
}

func SetupReaper() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGCHLD)

	go func() {
		for range sigCh {
			reapZombies()
		}
	}()
}

func reapZombies() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
		if pid <= 0 || err != nil {
			break
		}
		if val, ok := trackedPIDs.LoadAndDelete(pid); ok {
			// This is a managed child process — forward exit status
			// to the runner instead of silently discarding it.
			ch := val.(chan ReapResult)
			exitCode := 0
			if status.Exited() {
				exitCode = status.ExitStatus()
			} else if status.Signaled() {
				exitCode = 128 + int(status.Signal())
			}
			ch <- ReapResult{ExitStatus: exitCode}
			logger.Debug("forwarded exit status for managed process, pid=%d, code=%d", pid, exitCode)
		} else {
			logger.Debug("reaped zombie process, pid=%d", pid)
		}
	}
}
