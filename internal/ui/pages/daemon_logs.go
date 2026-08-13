// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	daemonLogsPageDownKeys = key.NewBinding(key.WithKeys("pgdown", "pagedown"))
	daemonLogsPageUpKeys   = key.NewBinding(key.WithKeys("pgup", "pageup"))
	daemonLogsHomeKeys     = key.NewBinding(key.WithKeys("home"))
	daemonLogsEndKeys      = key.NewBinding(key.WithKeys("end"))
	daemonLogsToggleKeys   = key.NewBinding(key.WithKeys("t", "T"))
	daemonLogsFollowKeys   = key.NewBinding(key.WithKeys("f", "F"))
)

type daemonLogViewMode int

const (
	daemonLogViewParsed daemonLogViewMode = iota
	daemonLogViewRaw
)

type DaemonLogsModel struct {
	lines     []string
	emptyHint string
	source    string
	offset    int
	width     int
	height    int
	mode      daemonLogViewMode
	follow    bool
	cancelled bool
}

type parsedLogLine struct {
	Time      string
	Level     string
	Component string
	Message   string
	Extra     string
	Prefix    string
	Kind      string
	Raw       string
	Parsed    bool
}

func NewDaemonLogs() DaemonLogsModel {
	return DaemonLogsModel{mode: daemonLogViewParsed, follow: true}
}

func (d DaemonLogsModel) Init() tea.Cmd { return nil }

func (d DaemonLogsModel) Update(msg tea.Msg) (DaemonLogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			d.cancelled = true
		case key.Matches(msg, pageUpKeys):
			d.follow = false
			d.scrollBy(-1)
		case key.Matches(msg, pageDownKeys):
			d.scrollBy(1)
			if d.offset >= d.maxOffset() {
				d.follow = true
			}
		case key.Matches(msg, daemonLogsPageUpKeys):
			d.follow = false
			d.scrollBy(-d.visibleLines())
		case key.Matches(msg, daemonLogsPageDownKeys):
			d.scrollBy(d.visibleLines())
			if d.offset >= d.maxOffset() {
				d.follow = true
			}
		case key.Matches(msg, daemonLogsHomeKeys):
			d.follow = false
			d.offset = 0
		case key.Matches(msg, daemonLogsEndKeys):
			d.follow = true
			d.offset = d.maxOffset()
		case key.Matches(msg, daemonLogsToggleKeys):
			if d.mode == daemonLogViewParsed {
				d.mode = daemonLogViewRaw
			} else {
				d.mode = daemonLogViewParsed
			}
		case key.Matches(msg, daemonLogsFollowKeys):
			d.follow = !d.follow
			if d.follow {
				d.offset = d.maxOffset()
			}
		}
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
	}
	return d, nil
}

func (d DaemonLogsModel) View() string {
	contentWidth := styles.Width - 12
	if d.width > 0 && d.width < styles.Width {
		contentWidth = d.width - 12
	}
	if contentWidth < 20 {
		contentWidth = 20
	}

	rows := []string{styles.TitleStyle.Render("Daemon Logs")}
	rows = append(rows, d.metaLine(contentWidth))
	rows = append(rows, "")

	if len(d.lines) == 0 {
		hint := d.emptyHint
		if hint == "" {
			hint = "No daemon logs yet."
		}
		rows = append(rows, styles.MutedStyle.Render(hint))
	} else {
		start := d.offset
		end := start + d.visibleLines()
		if end > len(d.lines) {
			end = len(d.lines)
		}
		for _, line := range d.lines[start:end] {
			rows = append(rows, d.renderLine(line, contentWidth))
		}
	}

	followLabel := "OFF"
	if d.follow {
		followLabel = "ON"
	}
	viewLabel := "parsed"
	if d.mode == daemonLogViewRaw {
		viewLabel = "raw"
	}
	footer := fmt.Sprintf("↑↓ Line • PgUp/PgDn Page • Home/End • T %s • F Follow %s • Esc Back", viewLabel, followLabel)
	rows = append(rows, "", styles.MutedStyle.Render(footer))
	return styles.ThemedBoxStyle().Width(styles.Width).Padding(2, 4).Render(strings.Join(rows, "\n"))
}

func (d *DaemonLogsModel) SetLines(lines []string) {
	prevAtEnd := d.follow || d.offset >= d.maxOffset()
	d.lines = append([]string(nil), lines...)
	if d.follow || prevAtEnd {
		d.offset = d.maxOffset()
	}
	if d.offset > d.maxOffset() {
		d.offset = d.maxOffset()
	}
}

func (d *DaemonLogsModel) SetEmptyHint(hint string) { d.emptyHint = hint }
func (d *DaemonLogsModel) SetSource(source string)  { d.source = source }
func (d DaemonLogsModel) Cancelled() bool           { return d.cancelled }
func (d *DaemonLogsModel) Reset()                   { d.cancelled = false }

func (d *DaemonLogsModel) scrollBy(delta int) {
	d.offset += delta
	if d.offset < 0 {
		d.offset = 0
	}
	if d.offset > d.maxOffset() {
		d.offset = d.maxOffset()
	}
}

func (d DaemonLogsModel) visibleLines() int {
	if d.height <= 0 {
		return 18
	}
	visible := d.height - 15
	if visible < 6 {
		visible = 6
	}
	return visible
}

func (d DaemonLogsModel) maxOffset() int {
	visible := d.visibleLines()
	if len(d.lines) <= visible {
		return 0
	}
	return len(d.lines) - visible
}

func (d DaemonLogsModel) metaLine(width int) string {
	view := "parsed"
	if d.mode == daemonLogViewRaw {
		view = "raw"
	}

	total := len(d.lines)
	vis := d.visibleLines()

	followTag := ""
	if d.follow {
		followTag = styles.SuccessStyle.Render(" ↓ FOLLOW")
	}

	if total == 0 {
		return fmt.Sprintf("Source: %s  View: %s%s", d.fallbackSource(), view, followTag)
	}

	start := d.offset + 1
	end := d.offset + vis
	if end > total {
		end = total
	}

	bar := renderScrollbar(d.offset, d.maxOffset(), 20)
	return fmt.Sprintf("%s  %s  %s%s",
		styles.MutedStyle.Render(fmt.Sprintf("%s [%d-%d/%d]", d.fallbackSource(), start, end, total)),
		styles.MutedStyle.Render("View:"+view),
		bar,
		followTag,
	)
}

func renderScrollbar(offset, maxOffset, width int) string {
	if maxOffset <= 0 {
		return styles.MutedStyle.Render("[" + strings.Repeat("█", width) + "]")
	}

	frac := float64(offset) / float64(maxOffset)
	thumbPos := int(frac * float64(width-1))
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos >= width {
		thumbPos = width - 1
	}

	var track strings.Builder
	for i := 0; i < width; i++ {
		if i == thumbPos {
			track.WriteString("█")
		} else {
			track.WriteString("░")
		}
	}

	return styles.TitleStyle.Render(track.String())
}

func (d DaemonLogsModel) renderLine(line string, width int) string {
	if d.mode == daemonLogViewRaw {
		return truncatePlain(line, width)
	}
	parsed := parseDaemonLogLine(line)
	if !parsed.Parsed {
		return truncatePlain(parsed.Raw, width)
	}
	if parsed.Kind == "systemd" {
		return renderSystemdLogLine(parsed, width)
	}

	timeWidth := 8
	levelWidth := 5
	compWidth := 16
	sep := 1
	messageWidth := width - timeWidth - levelWidth - compWidth - sep*3
	if messageWidth < 20 {
		messageWidth = 20
	}

	timeCol := styles.MutedStyle.Render(fitCellLeft(parsed.Time, timeWidth))
	levelCol := renderLogLevel(parsed.Level, levelWidth)
	componentCol := renderLogComponent(parsed.Component, compWidth)

	msgText := parsed.Message
	if parsed.Extra != "" {
		msgText = msgText + " " + parsed.Extra
	}
	messageCol := renderLogMessage(parsed.Level, parsed.Component, msgText, messageWidth)

	return timeCol + " " + levelCol + " " + componentCol + " " + messageCol
}

func (d DaemonLogsModel) fallbackSource() string {
	if strings.TrimSpace(d.source) == "" {
		return "unknown"
	}
	return d.source
}

func parseDaemonLogLine(line string) parsedLogLine {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return parsedLogLine{Raw: line}
	}

	payload := trimmed

	if len(payload) > 9 && payload[2] == ':' && payload[5] == ':' && payload[8] == ' ' {
		payload = payload[9:]
	}

	if idx := strings.Index(payload, "]: "); idx >= 0 {
		payload = strings.TrimSpace(payload[idx+3:])
	}

	if strings.HasPrefix(payload, "{") {
		return parseJSONLogLine(payload, trimmed)
	}

	parts := strings.Fields(payload)
	if len(parts) < 4 {
		if len(parts) > 0 {
			return parsedLogLine{
				Time:      "",
				Level:     "INFO",
				Component: "system",
				Message:   strings.Join(parts, " "),
				Kind:      "derod",
				Raw:       trimmed,
				Parsed:    true,
			}
		}
		return parseSystemdLogLine(trimmed)
	}
	if !looksLikeTime(parts[1]) {
		return parseSystemdLogLine(trimmed)
	}
	level := strings.ToUpper(parts[2])
	component := parts[3]
	message := ""
	if len(parts) > 4 {
		message = strings.Join(parts[4:], " ")
	}
	return parsedLogLine{
		Time:      parts[1],
		Level:     level,
		Component: component,
		Message:   message,
		Kind:      "derod",
		Raw:       trimmed,
		Parsed:    true,
	}
}

func parseJSONLogLine(payload, raw string) parsedLogLine {
	var entry struct {
		L string `json:"L"`
		T string `json:"T"`
		N string `json:"N"`
		M string `json:"M"`
	}
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		return parsedLogLine{Raw: raw}
	}

	timeStr := extractTimeFromISO(entry.T)
	extra := extractJSONExtras(payload)

	return parsedLogLine{
		Time:      timeStr,
		Level:     entry.L,
		Component: entry.N,
		Message:   entry.M,
		Extra:     extra,
		Kind:      "derod",
		Raw:       raw,
		Parsed:    true,
	}
}

func extractTimeFromISO(iso string) string {
	if len(iso) < 19 {
		return iso
	}
	timePart := iso[11:19]
	return timePart
}

func extractJSONExtras(payload string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return ""
	}

	skip := map[string]bool{"L": true, "T": true, "N": true, "C": true, "M": true}
	var parts []string
	for k, v := range raw {
		if skip[k] {
			continue
		}
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			parts = append(parts, k+"="+s)
		} else {
			parts = append(parts, k+"="+string(v))
		}
	}
	if len(parts) == 0 {
		return ""
	}

	result := strings.Join(parts, " ")
	if len(result) > 60 {
		return result[:57] + "..."
	}
	return result
}

func parseSystemdLogLine(line string) parsedLogLine {
	trimmed := strings.TrimSpace(line)
	idx := strings.Index(trimmed, ": ")
	if idx < 0 {
		return parsedLogLine{Raw: trimmed}
	}
	prefix := strings.TrimSpace(trimmed[:idx+1])
	message := strings.TrimSpace(trimmed[idx+2:])
	parts := strings.Fields(trimmed)
	timeVal := ""
	if len(parts) >= 3 && looksLikeTime(parts[2]) {
		timeVal = parts[2]
	}
	if prefix == "" || message == "" {
		return parsedLogLine{Raw: trimmed}
	}
	return parsedLogLine{
		Time:    timeVal,
		Prefix:  prefix,
		Message: message,
		Kind:    "systemd",
		Raw:     trimmed,
		Parsed:  true,
	}
}

func looksLikeTime(v string) bool {
	if len(v) != 8 {
		return false
	}
	return v[2] == ':' && v[5] == ':'
}

func renderLogLevel(level string, width int) string {
	text := fitCellLeft(level, width)
	switch level {
	case "DEBUG":
		return styles.MutedStyle.Render(text)
	case "ERROR", "FATAL":
		return styles.ErrorStyle.Render(text)
	case "WARN", "WARNING":
		return styles.WarningStyle.Render(text)
	case "INFO":
		return styles.TextStyle.Render(text)
	default:
		return styles.MutedStyle.Render(text)
	}
}

func renderLogComponent(component string, width int) string {
	short := component
	dotIdx := strings.LastIndex(component, ".")
	if dotIdx >= 0 && len(component)-dotIdx-1 > 2 {
		parts := strings.Split(component, ".")
		if len(parts) > 2 {
			short = parts[0] + "." + parts[len(parts)-1]
		}
	}
	text := fitCellLeft(short, width)
	switch strings.ToUpper(strings.SplitN(component, ".", 2)[0]) {
	case "RPC":
		return styles.SimulatorStyle.Render(text)
	case "P2P":
		return styles.SuccessStyle.Render(text)
	case "MEMPOOL", "REGPOOL":
		return styles.WarningStyle.Render(text)
	case "CORE":
		return styles.TitleStyle.Render(text)
	case "GETWORK":
		return styles.SimulatorStyle.Render(text)
	default:
		return styles.TextStyle.Render(text)
	}
}

func renderLogMessage(level, component, message string, width int) string {
	text := truncatePlain(message, width)
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"), strings.Contains(lower, "panic"):
		return styles.ErrorStyle.Render(text)
	case strings.Contains(lower, "backing off"):
		return styles.MutedStyle.Render(text)
	case strings.Contains(lower, "bootstrap in progress"):
		return renderBootstrapMessage(text)
	case strings.Contains(lower, "started"), strings.Contains(lower, "listening"), strings.Contains(lower, "will listen"):
		return styles.SuccessStyle.Render(text)
	case level == "WARN", level == "WARNING":
		return styles.WarningStyle.Render(text)
	case level == "ERROR", level == "FATAL":
		return styles.ErrorStyle.Render(text)
	case level == "DEBUG":
		return styles.MutedStyle.Render(text)
	default:
		return styles.TextStyle.Render(text)
	}
}

func renderBootstrapMessage(text string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "percent")
	if idx < 0 {
		return styles.TextStyle.Render(text)
	}

	prefix := text[:idx]
	var pctStr string
	for i := idx + 8; i < len(text); i++ {
		if text[i] >= '0' && text[i] <= '9' || text[i] == '.' || text[i] == '-' {
			pctStr += string(text[i])
		} else if pctStr != "" {
			break
		}
	}

	if pctStr == "" {
		return styles.TextStyle.Render(text)
	}

	pct := 0.0
	fmt.Sscanf(pctStr, "%f", &pct)

	barWidth := 8
	filled := int(pct / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	return styles.TextStyle.Render(prefix) + "percent:" + styles.SuccessStyle.Render(bar) + styles.SuccessStyle.Render(pctStr+"%")
}

func renderSystemdLogLine(parsed parsedLogLine, width int) string {
	prefixWidth := width / 3
	if prefixWidth < 24 {
		prefixWidth = 24
	}
	if prefixWidth > 42 {
		prefixWidth = 42
	}
	messageWidth := width - prefixWidth - 1
	if messageWidth < 10 {
		messageWidth = 10
	}
	prefix := styles.MutedStyle.Render(fitCellLeft(parsed.Prefix, prefixWidth))
	message := renderSystemdMessage(parsed.Message, messageWidth)
	return prefix + " " + message
}

func renderSystemdMessage(message string, width int) string {
	trimmed := truncatePlain(message, width)
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "failed"), strings.Contains(lower, "error"), strings.Contains(lower, "deactivated"):
		return styles.ErrorStyle.Render(trimmed)
	case strings.Contains(lower, "stopping"), strings.Contains(lower, "stopped"):
		return styles.WarningStyle.Render(trimmed)
	case strings.Contains(lower, "started"), strings.Contains(lower, "active"), strings.Contains(lower, "running"):
		return styles.SuccessStyle.Render(trimmed)
	default:
		return styles.TextStyle.Render(trimmed)
	}
}

func truncatePlain(text string, width int) string {
	if width < 1 {
		return ""
	}
	line := strings.ReplaceAll(text, "\n", " ")
	line = strings.ReplaceAll(line, "\r", "")
	if len(line) > width {
		return line[:width-3] + "..."
	}
	return line + strings.Repeat(" ", width-len(line))
}
