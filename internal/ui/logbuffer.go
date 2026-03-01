// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// LogBuffer provides a bridge between the structured logger and UI display
type LogBuffer struct {
	mu       sync.RWMutex
	entries  []log.LogEntry
	capacity int
	writer   io.Writer
}

// NewLogBuffer creates a new log buffer with the specified capacity
func NewLogBuffer(capacity int, fileWriter io.Writer) *LogBuffer {
	return &LogBuffer{
		entries:  make([]log.LogEntry, 0, capacity),
		capacity: capacity,
		writer:   fileWriter,
	}
}

// Write implements io.Writer - receives logfmt lines and parses them
func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	// Write to file first
	if lb.writer != nil {
		_, _ = lb.writer.Write(p)
	}

	// Parse structured log line and store
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		entry := lb.parseLogfmtEntry(trimmed)
		lb.mu.Lock()
		lb.entries = append(lb.entries, entry)
		if len(lb.entries) > lb.capacity {
			lb.entries = lb.entries[1:]
		}
		lb.mu.Unlock()
	}

	return len(p), nil
}

// parseLogfmtEntry parses a logfmt line into a structured entry
// Format: "2006-01-02T15:04:05.000 level=DEBUG component=wallet event=open msg=..."
func (lb *LogBuffer) parseLogfmtEntry(line string) log.LogEntry {
	entry := log.LogEntry{
		Timestamp: time.Now(),
		Level:     log.LevelInfo,
		Component: "unknown",
		Event:     "log",
		Message:   line,
		Fields:    make(map[string]string),
	}

	// Extract timestamp if present
	if len(line) > 23 && line[4] == '-' && line[7] == '-' {
		if ts, err := time.Parse("2006-01-02T15:04:05.000", line[:23]); err == nil {
			entry.Timestamp = ts
			line = strings.TrimSpace(line[23:])
		}
	}

	// Parse key=value pairs
	for len(line) > 0 {
		// Find next key
		keyEnd := strings.IndexAny(line, "= \t")
		if keyEnd <= 0 {
			break
		}
		key := line[:keyEnd]
		if keyEnd >= len(line) || line[keyEnd] != '=' {
			// No value for this key, skip
			line = strings.TrimLeft(line[keyEnd:], " \t")
			continue
		}

		// Find value
		line = line[keyEnd+1:]
		var value string

		if len(line) > 0 && line[0] == '"' {
			// Quoted value
			line = line[1:]
			endQuote := strings.Index(line, "\"")
			if endQuote >= 0 {
				value = strings.ReplaceAll(line[:endQuote], "\\\"", "\"")
				line = line[endQuote+1:]
			} else {
				value = line
				line = ""
			}
		} else {
			// Unquoted value
			valEnd := strings.IndexAny(line, " \t")
			if valEnd >= 0 {
				value = line[:valEnd]
				line = line[valEnd:]
			} else {
				value = line
				line = ""
			}
		}

		// Assign to entry fields
		switch key {
		case "level":
			entry.Level = parseLevel(value)
		case "component":
			entry.Component = value
		case "event":
			entry.Event = value
		case "msg":
			entry.Message = value
		default:
			entry.Fields[key] = value
		}

		line = strings.TrimLeft(line, " \t")
	}

	return entry
}

func parseLevel(s string) log.LogLevel {
	switch strings.ToUpper(s) {
	case "DEBUG", "DBG":
		return log.LevelDebug
	case "INFO", "INF":
		return log.LevelInfo
	case "WARN", "WARNING", "WRN":
		return log.LevelWarn
	case "ERROR", "ERR":
		return log.LevelError
	default:
		return log.LevelInfo
	}
}

// GetEntries returns a copy of current entries
func (lb *LogBuffer) GetEntries() []log.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]log.LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// GetLastN returns the last n entries
func (lb *LogBuffer) GetLastN(n int) []log.LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n >= len(lb.entries) {
		result := make([]log.LogEntry, len(lb.entries))
		copy(result, lb.entries)
		return result
	}

	result := make([]log.LogEntry, n)
	copy(result, lb.entries[len(lb.entries)-n:])
	return result
}

// Clear clears all entries
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.entries = lb.entries[:0]
}

// logUpdateMsg is sent when new logs are available
type logUpdateMsg struct{}

// Global log buffer instance
var globalLogBuffer *LogBuffer

// globalLogFile tracks the current log file handle for safe closure on toggle
var globalLogFile *os.File

// CloseLogFile closes the current log file handle safely
func CloseLogFile() {
	if globalLogFile != nil {
		globalLogFile.Close()
		globalLogFile = nil
	}
}

// LogWriter wraps the log buffer for use with log package
type LogWriter struct {
	buffer  *LogBuffer
	program *tea.Program
}

// NewLogWriter creates a new log writer that writes to both file and buffer
func NewLogWriter(buffer *LogBuffer) *LogWriter {
	return &LogWriter{
		buffer: buffer,
	}
}

// SetProgram sets the tea program for sending update messages
func (lw *LogWriter) SetProgram(p *tea.Program) {
	lw.program = p
}

// Write implements io.Writer
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	n, err = lw.buffer.Write(p)

	// Notify UI of new log entry
	if lw.program != nil {
		lw.program.Send(logUpdateMsg{})
	}

	return n, err
}

// IsHighSignal returns true if this log entry is important enough to show in the debug panel
func IsHighSignal(entry log.LogEntry) bool {
	// Always show errors and warnings
	if entry.Level >= log.LevelWarn {
		return true
	}

	// Show important lifecycle events
	if entry.Event != "" && entry.Event != "log" {
		// Skip noisy polling events
		if strings.Contains(entry.Event, "poll") ||
			strings.Contains(entry.Event, "tick") ||
			strings.Contains(entry.Event, "sync") {
			return false
		}
		return true
	}

	// Skip raw legacy logs that weren't structured
	if entry.Component == "legacy" || entry.Component == "unknown" {
		return false
	}

	return false
}

// FormatLogEntry formats a log entry for display in the debug console
func FormatLogEntry(entry log.LogEntry, maxWidth int) string {
	timestamp := entry.Timestamp.Format("15:04:05")

	// Build message with event and component context
	var msgParts []string

	// Add component:event prefix for structured logs
	if entry.Component != "" && entry.Component != "unknown" && entry.Component != "legacy" {
		if entry.Event != "" && entry.Event != "log" {
			msgParts = append(msgParts, fmt.Sprintf("%s.%s", entry.Component, entry.Event))
		} else {
			msgParts = append(msgParts, entry.Component)
		}
	}

	if entry.Message != "" {
		msgParts = append(msgParts, entry.Message)
	}

	message := strings.Join(msgParts, " ")
	if message == "" {
		message = entry.Message
	}
	if message == "" {
		message = "(no message)"
	}

	// Add key fields (excluding internal ones and keeping it concise)
	var fields []string
	priorityFields := []string{"error", "err", "txid", "amount", "network", "duration", "file"}

	// First add priority fields
	for _, k := range priorityFields {
		if v, ok := entry.Fields[k]; ok {
			if len(v) > 15 {
				v = v[:12] + "..."
			}
			fields = append(fields, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Then add other fields (limit to 2 more)
	otherCount := 0
	for k, v := range entry.Fields {
		if otherCount >= 2 {
			break
		}
		// Skip already added and internal fields
		skip := false
		for _, pk := range priorityFields {
			if k == pk {
				skip = true
				break
			}
		}
		if k == "ts" || k == "level" || k == "component" || k == "event" || k == "msg" {
			skip = true
		}
		if skip {
			continue
		}

		if len(v) > 15 {
			v = v[:12] + "..."
		}
		fields = append(fields, fmt.Sprintf("%s=%s", k, v))
		otherCount++
	}

	if len(fields) > 0 {
		message += " | " + strings.Join(fields, " ")
	}

	// Truncate message if needed
	visibleLen := len(timestamp) + 3 + len(message)
	if visibleLen > maxWidth && maxWidth > 30 {
		maxMsgLen := maxWidth - len(timestamp) - 6
		if maxMsgLen > 3 {
			message = message[:maxMsgLen] + "..."
		}
	}

	return fmt.Sprintf("%s %s", timestamp, message)
}

// SetupLogBuffer initializes the global log buffer and redirects log output.
// The provided file is tracked so it can be closed safely before opening a new log file.
func SetupLogBuffer(file *os.File) *LogBuffer {
	// Close any previously opened log file to avoid descriptor leaks
	CloseLogFile()
	globalLogFile = file

	globalLogBuffer = NewLogBuffer(300, file)

	// Create a multiwriter that writes to buffer (which also writes to file)
	logWriter := NewLogWriter(globalLogBuffer)
	log.SetOutput(logWriter)

	return globalLogBuffer
}

// GetLogBuffer returns the global log buffer
func GetLogBuffer() *LogBuffer {
	return globalLogBuffer
}

// LogBufferWriter is an io.Writer that captures output for display
type LogBufferWriter struct {
	buffer *bytes.Buffer
	mu     sync.Mutex
}

// NewLogBufferWriter creates a new log buffer writer
func NewLogBufferWriter() *LogBufferWriter {
	return &LogBufferWriter{
		buffer: &bytes.Buffer{},
	}
}

// Write implements io.Writer
func (w *LogBufferWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

// String returns the buffer contents
func (w *LogBufferWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

// Clear clears the buffer
func (w *LogBufferWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer.Reset()
}

// ColorForLevel returns the appropriate style color for a log level
func ColorForLevel(level log.LogLevel) color.Color {
	switch level {
	case log.LevelError:
		return styles.ColorError
	case log.LevelWarn:
		return styles.ColorWarning
	case log.LevelDebug:
		return styles.ColorPrimary
	default:
		return styles.ColorText
	}
}
