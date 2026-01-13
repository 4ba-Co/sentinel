package signal

import (
	"os"
	"os/signal"
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

func SetupReaper() {
	// Setup SIGCHLD handler for zombie reaping (PID 1 mode)
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
		logger.Debug("reaped zombie process, pid=%d", pid)
	}
}
