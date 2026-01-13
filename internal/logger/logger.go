package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Logger struct {
	mu       sync.Mutex
	out      io.Writer
	jsonMode bool
}

type logEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

var defaultLogger = &Logger{out: os.Stderr, jsonMode: false}

func SetOutput(w io.Writer) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.out = w
}

func SetJSONMode(enabled bool) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.jsonMode = enabled
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	now := time.Now().Format(time.RFC3339)

	if l.jsonMode {
		entry := logEntry{
			Time:    now,
			Level:   string(level),
			Message: msg,
		}
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.out, string(data))
	} else {
		fmt.Fprintf(l.out, "[%s] [%s] %s\n", now, level, msg)
	}
}

func Debug(format string, args ...interface{}) {
	defaultLogger.log(LevelDebug, format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.log(LevelInfo, format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.log(LevelWarn, format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.log(LevelError, format, args...)
}
