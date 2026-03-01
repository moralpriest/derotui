// Copyright 2017-2026 DERO Project. All rights reserved.

package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel represents logging levels
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
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

// toSlogLevel converts LogLevel to slog.Level
func (l LogLevel) toSlogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LogEntry represents a structured log entry for UI display
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Component string
	Event     string
	Message   string
	Fields    map[string]string
}

// Logger provides structured logging wrapping log/slog with UI buffer support
type Logger struct {
	mu     sync.RWMutex
	logger *slog.Logger
	out    io.Writer
	level  LogLevel
	buffer []LogEntry
	maxBuf int
}

// defaultLogger is the global logger instance
var defaultLogger *Logger

func init() {
	defaultLogger = New(io.Discard, LevelDebug)
}

// New creates a new logger wrapping slog
func New(out io.Writer, level LogLevel) *Logger {
	opts := &slog.HandlerOptions{
		Level: level.toSlogLevel(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("2006-01-02T15:04:05.000"))
				}
			}
			return a
		},
	}
	handler := slog.NewTextHandler(out, opts)
	logger := slog.New(handler)

	return &Logger{
		logger: logger,
		out:    out,
		level:  level,
		buffer: make([]LogEntry, 0, 300),
		maxBuf: 300,
	}
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(out io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
	opts := &slog.HandlerOptions{
		Level: l.level.toSlogLevel(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("2006-01-02T15:04:05.000"))
				}
			}
			return a
		},
	}
	l.logger = slog.New(slog.NewTextHandler(out, opts))
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
	opts := &slog.HandlerOptions{
		Level: level.toSlogLevel(),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("2006-01-02T15:04:05.000"))
				}
			}
			return a
		},
	}
	l.logger = slog.New(slog.NewTextHandler(l.out, opts))
}

// log writes a structured log entry using slog
func (l *Logger) log(level LogLevel, component, event, message string, fields ...string) {
	if level < l.level {
		return
	}

	// Build slog attributes
	attrs := []slog.Attr{
		slog.String("component", component),
		slog.String("event", event),
	}

	// Add fields as attributes
	for i := 0; i < len(fields)-1; i += 2 {
		attrs = append(attrs, slog.String(fields[i], fields[i+1]))
	}

	// Create log entry for buffer
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Component: component,
		Event:     event,
		Message:   message,
		Fields:    make(map[string]string),
	}
	for i := 0; i < len(fields)-1; i += 2 {
		entry.Fields[fields[i]] = fields[i+1]
	}

	// Log using slog with attributes
	ctx := context.Background()
	l.logger.LogAttrs(ctx, level.toSlogLevel(), message, attrs...)

	// Buffer for UI display
	l.mu.Lock()
	l.buffer = append(l.buffer, entry)
	if len(l.buffer) > l.maxBuf {
		l.buffer = l.buffer[len(l.buffer)-l.maxBuf:]
	}
	l.mu.Unlock()
}

// formatLogEntry formats entry as logfmt for display
func formatLogEntry(e LogEntry) string {
	var b strings.Builder
	b.WriteString(e.Timestamp.Format("2006-01-02T15:04:05.000"))
	b.WriteString(" level=")
	b.WriteString(e.Level.String())
	b.WriteString(" component=")
	b.WriteString(escapeValue(e.Component))
	b.WriteString(" event=")
	b.WriteString(escapeValue(e.Event))
	if e.Message != "" {
		b.WriteString(" msg=\"")
		b.WriteString(escapeString(e.Message))
		b.WriteString("\"")
	}
	for k, v := range e.Fields {
		b.WriteString(" ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(escapeValue(v))
	}
	return b.String()
}

// escapeValue escapes values for logfmt
func escapeValue(s string) string {
	if s == "" {
		return "\"\""
	}
	if strings.ContainsAny(s, " =\"\n\t") {
		return fmt.Sprintf("\"%s\"", escapeString(s))
	}
	return s
}

// escapeString escapes quotes within strings
func escapeString(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}

// GetBuffer returns recent entries (for UI debug console)
func (l *Logger) GetBuffer() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]LogEntry, len(l.buffer))
	copy(result, l.buffer)
	return result
}

// GetEntries returns entries as formatted strings (legacy compat)
func (l *Logger) GetEntries() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]string, len(l.buffer))
	for i, e := range l.buffer {
		result[i] = formatLogEntry(e)
	}
	return result
}

// ClearBuffer clears the buffer
func (l *Logger) ClearBuffer() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buffer = l.buffer[:0]
}

// Public API methods
func (l *Logger) Debug(component, event, message string, fields ...string) {
	l.log(LevelDebug, component, event, message, fields...)
}

func (l *Logger) Info(component, event, message string, fields ...string) {
	l.log(LevelInfo, component, event, message, fields...)
}

func (l *Logger) Warn(component, event, message string, fields ...string) {
	l.log(LevelWarn, component, event, message, fields...)
}

func (l *Logger) Error(component, event, message string, fields ...string) {
	l.log(LevelError, component, event, message, fields...)
}

// Package-level functions using default logger
func SetOutput(out io.Writer) {
	defaultLogger.SetOutput(out)
}

func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

func Debug(component, event, message string, fields ...string) {
	defaultLogger.Debug(component, event, message, fields...)
}

func Info(component, event, message string, fields ...string) {
	defaultLogger.Info(component, event, message, fields...)
}

func Warn(component, event, message string, fields ...string) {
	defaultLogger.Warn(component, event, message, fields...)
}

func Error(component, event, message string, fields ...string) {
	defaultLogger.Error(component, event, message, fields...)
}

func GetBuffer() []LogEntry {
	return defaultLogger.GetBuffer()
}

func ClearBuffer() {
	defaultLogger.ClearBuffer()
}

// Setup initializes logging for the application
func Setup(debug bool, logDir string) (string, error) {
	if !debug {
		SetOutput(io.Discard)
		return "", nil
	}

	if logDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		logDir = cwd
	}

	logPath := filepath.Join(logDir, "derotui-debug.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %w", err)
	}

	SetOutput(logFile)
	Info("log", "init", "Logging initialized", "path", logPath)
	return logPath, nil
}

// WithContext returns a logger bound to context (for future tracing)
func WithContext(ctx context.Context) *Logger {
	return defaultLogger
}

// Redaction helpers for safe logging

// TruncateAddress truncates crypto addresses for safe logging
func TruncateAddress(addr string) string {
	if len(addr) <= 16 {
		return addr
	}
	return addr[:8] + "..." + addr[len(addr)-8:]
}

// TruncateID truncates IDs (txids, hashes) for safe logging
func TruncateID(id string) string {
	if len(id) <= 16 {
		return id
	}
	return id[:8] + "..." + id[len(id)-8:]
}

// MaskSeed returns a masked version of seed phrase (first word only)
func MaskSeed(seed string) string {
	words := strings.Fields(seed)
	if len(words) == 0 {
		return "[empty]"
	}
	return words[0] + " ... [" + fmt.Sprintf("%d", len(words)) + " words]"
}

// FormatAmount formats atomic DERO amount for logging
func FormatAmount(atomic uint64) string {
	return fmt.Sprintf("%.5f", float64(atomic)/100000.0)
}

// FormatDuration formats duration for logging
func FormatDuration(d time.Duration) string {
	return d.Round(time.Millisecond).String()
}

// Compatibility with standard log package
func Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	level := LevelInfo
	if strings.Contains(msg, "[DEBUG") || strings.Contains(msg, "debug") {
		level = LevelDebug
	} else if strings.Contains(msg, "[ERROR") || strings.Contains(msg, "error") || strings.Contains(msg, "Error") {
		level = LevelError
	} else if strings.Contains(msg, "[WARN") || strings.Contains(msg, "warn") {
		level = LevelWarn
	}
	defaultLogger.log(level, "legacy", "log", msg)
}

func Println(v ...interface{}) {
	Printf("%s", fmt.Sprintln(v...))
}

// Hook to integrate with standard log package
func RedirectStandardLog() {
	log.SetOutput(&stdLogAdapter{})
	log.SetFlags(0)
}

type stdLogAdapter struct{}

func (a *stdLogAdapter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		Printf("%s", msg)
	}
	return len(p), nil
}
