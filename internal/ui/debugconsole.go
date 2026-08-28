// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

// Debug console helpers. Log entries come from the structured logger's
// internal buffer (derolog.GetBuffer); the log file itself is owned by
// internal/log.Setup, so nothing here touches files.

import (
	"fmt"
	"strings"

	derolog "github.com/deroproject/dero-wallet-cli/internal/log"
)

// IsHighSignal returns true if this log entry is important enough to show in
// the debug panel. ui.nav diagnostics are file-only (they fire on every
// keypress) so they never flood the console.
func IsHighSignal(entry derolog.LogEntry) bool {
	if entry.Component == "ui.nav" && entry.Event == "diag" {
		return false
	}
	if entry.Level >= derolog.LevelWarn {
		return true
	}
	// Daemon lifecycle + helper stdout always belong in the debug console.
	if strings.EqualFold(entry.Component, "daemon") {
		if strings.Contains(entry.Event, "poll") ||
			strings.Contains(entry.Event, "tick") ||
			strings.Contains(entry.Event, "refresh") {
			return false
		}
		return true
	}
	if entry.Event != "" && entry.Event != "log" {
		if strings.Contains(entry.Event, "poll") ||
			strings.Contains(entry.Event, "tick") ||
			strings.Contains(entry.Event, "refresh") {
			return false
		}
		return true
	}
	if entry.Component == "legacy" || entry.Component == "unknown" {
		return false
	}
	return false
}

// FormatLogEntry formats a log entry for display in the debug console
func FormatLogEntry(entry derolog.LogEntry, maxWidth int) string {
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
