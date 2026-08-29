// Copyright 2017-2026 DERO Project. All rights reserved.

package styles

import (
	_ "embed"
	"strings"
)

//go:embed dero_hex.ansi
var hexLogoANSI string

//go:embed dero_hex_small.ansi
var hexLogoSmallANSI string

func HexLogo() string {
	return trimANSIBlankLines(hexLogoANSI)
}

func HexLogoSmall() string {
	return trimANSIBlankLines(hexLogoSmallANSI)
}

func trimANSIBlankLines(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(stripANSI(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(stripANSI(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
