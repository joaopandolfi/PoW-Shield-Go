package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Options struct {
	Level       Level
	FilePath    string
	MaxSizeMB   int
	MaxFiles    int
	Compress    bool
	Timestamp   bool
	Caller      bool
	Stacktrace  bool
	Component   string
	Environment string
}

type Logger struct {
	slog       *slog.Logger
	opts       Options
	startTime  time.Time
	reqCount   int64
	errCount   int64
	lastErrors []errorEntry
}

type errorEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     Level     `json:"level"`
	Comp      string    `json:"component"`
	Message   string    `json:"message"`
	Stack     string    `json:"stack,omitempty"`
}

type Stats struct {
	Uptime       time.Duration `json:"uptime"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
}

var global *Logger

func Init(opts Options) *Logger {
	var writer io.Writer = os.Stdout

	if opts.FilePath != "" {
		os.MkdirAll(filepath.Dir(opts.FilePath), 0755)
		f, err := os.OpenFile(opts.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			writer = f
		}
	}

	var slvl slog.Level
	switch opts.Level {
	case LevelDebug:
		slvl = slog.LevelDebug
	case LevelInfo:
		slvl = slog.LevelInfo
	case LevelWarn:
		slvl = slog.LevelWarn
	case LevelError:
		slvl = slog.LevelError
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     slvl,
		AddSource: opts.Caller,
	})

	logger := slog.New(handler)
	if opts.Component != "" {
		logger = logger.With("component", opts.Component)
	}
	if opts.Environment != "" {
		logger = logger.With("env", opts.Environment)
	}

	global = &Logger{
		slog:      logger,
		opts:      opts,
		startTime: time.Now(),
	}

	return global
}

func logWithCaller(level Level, msg string, fields ...any) {
	_, file, line, ok := runtime.Caller(2)
	if ok {
		fields = append(fields, "caller", fmt.Sprintf("%s:%d", filepath.Base(file), line))
	}

	switch level {
	case LevelDebug:
		global.slog.Debug(msg, fields...)
	case LevelInfo:
		global.slog.Info(msg, fields...)
	case LevelWarn:
		global.slog.Warn(msg, fields...)
	case LevelError:
		global.errCount++
		global.slog.Error(msg, fields...)
		if global.opts.Stacktrace {
			stack := make([]byte, 2048)
			n := runtime.Stack(stack, false)
			fields = append(fields, "stack", string(stack[:n]))
			global.lastErrors = append(global.lastErrors, errorEntry{
				Timestamp: time.Now(), Level: level, Comp: global.opts.Component,
				Message: msg, Stack: string(stack[:n]),
			})
			if len(global.lastErrors) > 100 {
				global.lastErrors = global.lastErrors[len(global.lastErrors)-100:]
			}
		}
	}
}

func (l *Logger) Debug(msg string, fields ...any) {
	if global == nil {
		return
	}
	logWithCaller(LevelDebug, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...any) {
	if global == nil {
		return
	}
	global.reqCount++
	logWithCaller(LevelInfo, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...any) {
	if global == nil {
		return
	}
	logWithCaller(LevelWarn, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...any) {
	if global == nil {
		return
	}
	logWithCaller(LevelError, msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...any) {
	if global == nil {
		return
	}
	l.Error(msg, fields...)
	os.Exit(1)
}

func (l *Logger) StatsFunc() Stats {
	return Stats{
		Uptime:       time.Since(l.startTime),
		RequestCount: l.reqCount,
		ErrorCount:   l.errCount,
	}
}

func (l *Logger) GetLastErrors(count int) []errorEntry {
	if count <= 0 {
		count = 20
	}
	if len(global.lastErrors) < count {
		count = len(global.lastErrors)
	}
	if count <= 0 {
		return nil
	}
	if count > len(global.lastErrors) {
		count = len(global.lastErrors)
	}
	lastErrors := make([]errorEntry, count)
	start := len(global.lastErrors) - count
	for i := 0; i < count; i++ {
		lastErrors[i] = global.lastErrors[start+i]
	}
	return lastErrors
}

func Get() *Logger {
	return global
}

func (l *Logger) GetReqCount() int64 {
	return l.reqCount
}

func (l *Logger) GetErrCount() int64 {
	return l.errCount
}
