// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"bytes"
	"strings"
	"sync"
)

type LogBuffer struct {
	mu    sync.RWMutex
	lines []string
	max   int
}

func NewLogBuffer(max int) *LogBuffer {
	return &LogBuffer{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (b *LogBuffer) Write(p []byte) (n int, err error) {
	data := string(p)
	lines := strings.Split(strings.TrimRight(data, "\n"), "\n")

	b.mu.Lock()
	for _, line := range lines {
		if line == "" {
			continue
		}
		b.lines = append(b.lines, line)
		if len(b.lines) > b.max {
			b.lines = b.lines[len(b.lines)-b.max:]
		}
	}
	b.mu.Unlock()

	return len(p), nil
}

func (b *LogBuffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

func (b *LogBuffer) Clear() {
	b.mu.Lock()
	b.lines = b.lines[:0]
	b.mu.Unlock()
}

type LineWriter struct {
	buf *bytes.Buffer
	dst *LogBuffer
}

func (lb *LogBuffer) LineWriter() *LineWriter {
	return &LineWriter{buf: &bytes.Buffer{}, dst: lb}
}

func (lw *LineWriter) Write(p []byte) (n int, err error) {
	n, err = lw.buf.Write(p)
	if err != nil {
		return
	}
	for {
		line, err := lw.buf.ReadString('\n')
		if err != nil {
			lw.buf.WriteString(line)
			break
		}
		line = strings.TrimRight(line, "\n\r")
		if line != "" {
			lw.dst.Write([]byte(line))
		}
	}
	return
}
