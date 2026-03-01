// Copyright 2017-2026 DERO Project. All rights reserved.

package styles

import (
	"image/color"
	"strconv"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
)

// Layout constants for mouse handler positioning
const (
	SmallBoxHeight  = 20 // txdetails, daemon, network, xswd, history
	MediumBoxHeight = 25 // password, seed, qrcode, keyinput, welcome, integratedaddr
)

// Theme holds all color values for a theme
type Theme struct {
	ID        string
	Name      string
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Text      color.Color
	Muted     color.Color
	Success   color.Color
	Error     color.Color
	Warning   color.Color
	Border    color.Color
	Bg        color.Color
	CardBg    color.Color
	Testnet   color.Color
	Simulator color.Color
}

// Theme definitions
var themes = map[string]Theme{
	"neon": {
		ID:        "neon",
		Name:      "Neon",
		Primary:   lipgloss.Color("#BF40FF"),
		Secondary: lipgloss.Color("#000000"),
		Accent:    lipgloss.Color("#E879F9"),
		Text:      lipgloss.Color("#FFFFFF"),
		Muted:     lipgloss.Color("#9CA3AF"),
		Success:   lipgloss.Color("#10B981"),
		Error:     lipgloss.Color("#EF4444"),
		Warning:   lipgloss.Color("#F59E0B"),
		Border:    lipgloss.Color("#4C1D95"),
		Bg:        lipgloss.Color("#000000"),
		CardBg:    lipgloss.Color("#1E1B2E"),
		Testnet:   lipgloss.Color("#F59E0B"),
		Simulator: lipgloss.Color("#06B6D4"),
	},
	"matrix": {
		ID:        "matrix",
		Name:      "Matrix",
		Primary:   lipgloss.Color("#00FF41"),
		Secondary: lipgloss.Color("#000000"),
		Accent:    lipgloss.Color("#00CC33"),
		Text:      lipgloss.Color("#00FF41"),
		Muted:     lipgloss.Color("#008F11"),
		Success:   lipgloss.Color("#10B981"),
		Error:     lipgloss.Color("#FF0000"),
		Warning:   lipgloss.Color("#FFA500"),
		Border:    lipgloss.Color("#003B00"),
		Bg:        lipgloss.Color("#000000"),
		CardBg:    lipgloss.Color("#001100"),
		Testnet:   lipgloss.Color("#FFCC00"),
		Simulator: lipgloss.Color("#00FFFF"),
	},
	"amber-crt": {
		ID:        "amber-crt",
		Name:      "Amber CRT",
		Primary:   lipgloss.Color("#FFC107"),
		Secondary: lipgloss.Color("#1A0F00"),
		Accent:    lipgloss.Color("#FFEA00"),
		Text:      lipgloss.Color("#FFAB40"),
		Muted:     lipgloss.Color("#CC7722"),
		Success:   lipgloss.Color("#4CAF50"),
		Error:     lipgloss.Color("#FF5722"),
		Warning:   lipgloss.Color("#FF9800"),
		Border:    lipgloss.Color("#B8860B"),
		Bg:        lipgloss.Color("#080400"),
		CardBg:    lipgloss.Color("#2A1A05"),
		Testnet:   lipgloss.Color("#DAA520"),
		Simulator: lipgloss.Color("#FFD54F"),
	},
	"solarized-dark": {
		ID:        "solarized-dark",
		Name:      "Solarized Dark",
		Primary:   lipgloss.Color("#268BD2"),
		Secondary: lipgloss.Color("#002B36"),
		Accent:    lipgloss.Color("#6C71C4"),
		Text:      lipgloss.Color("#93A1A1"),
		Muted:     lipgloss.Color("#657B83"),
		Success:   lipgloss.Color("#2AA198"),
		Error:     lipgloss.Color("#DC322F"),
		Warning:   lipgloss.Color("#B58900"),
		Border:    lipgloss.Color("#073642"),
		Bg:        lipgloss.Color("#001B27"),
		CardBg:    lipgloss.Color("#052833"),
		Testnet:   lipgloss.Color("#CB4B16"),
		Simulator: lipgloss.Color("#6C71C4"),
	},
	"crimson-dark": {
		ID:        "crimson-dark",
		Name:      "Crimson Dark",
		Primary:   lipgloss.Color("#DC143C"),
		Secondary: lipgloss.Color("#1A0005"),
		Accent:    lipgloss.Color("#FF1744"),
		Text:      lipgloss.Color("#FF6B7A"),
		Muted:     lipgloss.Color("#8B3A3A"),
		Success:   lipgloss.Color("#00C853"),
		Error:     lipgloss.Color("#FF1744"),
		Warning:   lipgloss.Color("#FFB300"),
		Border:    lipgloss.Color("#8B0000"),
		Bg:        lipgloss.Color("#0A0002"),
		CardBg:    lipgloss.Color("#1D0508"),
		Testnet:   lipgloss.Color("#FF6D00"),
		Simulator: lipgloss.Color("#FF4081"),
	},
	"neon-pink": {
		ID:        "neon-pink",
		Name:      "Neon Pink",
		Primary:   lipgloss.Color("#FF2D95"),
		Secondary: lipgloss.Color("#0A0012"),
		Accent:    lipgloss.Color("#FF5DB8"),
		Text:      lipgloss.Color("#FFD6EA"),
		Muted:     lipgloss.Color("#A85C84"),
		Success:   lipgloss.Color("#00E676"),
		Error:     lipgloss.Color("#FF5252"),
		Warning:   lipgloss.Color("#FFB74D"),
		Border:    lipgloss.Color("#8A1F5A"),
		Bg:        lipgloss.Color("#0A0012"),
		CardBg:    lipgloss.Color("#1A0624"),
		Testnet:   lipgloss.Color("#FFAB40"),
		Simulator: lipgloss.Color("#EA80FC"),
	},
	"gruvbox-dark": {
		ID:        "gruvbox-dark",
		Name:      "Gruvbox Dark",
		Primary:   lipgloss.Color("#D79921"),
		Secondary: lipgloss.Color("#282828"),
		Accent:    lipgloss.Color("#FABD2F"),
		Text:      lipgloss.Color("#EBDBB2"),
		Muted:     lipgloss.Color("#928374"),
		Success:   lipgloss.Color("#B8BB26"),
		Error:     lipgloss.Color("#FB4934"),
		Warning:   lipgloss.Color("#FE8019"),
		Border:    lipgloss.Color("#504945"),
		Bg:        lipgloss.Color("#282828"),
		CardBg:    lipgloss.Color("#3C3836"),
		Testnet:   lipgloss.Color("#D65D0E"),
		Simulator: lipgloss.Color("#83A598"),
	},
}

// StringBuilderPool provides pooled strings.Builder instances for rendering.
// This reduces allocations during frequent TUI redraws.
var StringBuilderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

// GetBuilder returns a pooled strings.Builder.
// Call PutBuilder after use to return it to the pool.
func GetBuilder() *strings.Builder {
	return StringBuilderPool.Get().(*strings.Builder)
}

// PutBuilder returns a strings.Builder to the pool.
// The builder is reset before being returned.
func PutBuilder(b *strings.Builder) {
	b.Reset()
	StringBuilderPool.Put(b)
}

// CachedStyles holds pre-computed styles for common widths.
// These are rebuilt when the theme changes.
var CachedStyles struct {
	// Common box styles
	BoxWidth52 lipgloss.Style
	BoxWidth60 lipgloss.Style
	BoxWidth70 lipgloss.Style

	// Pre-computed separators (max reasonable width)
	Separator100 string
	Separator80  string
	Separator52  string

	// Pre-computed spacers
	Spacer10 string
	Spacer20 string
	Spacer30 string
}

// initCachedStyles pre-computes commonly used style variants.
// Called during package init and on theme change.
func initCachedStyles() {
	CachedStyles.BoxWidth52 = BoxStyle.Copy().Width(52)
	CachedStyles.BoxWidth60 = BoxStyle.Copy().Width(60)
	CachedStyles.BoxWidth70 = BoxStyle.Copy().Width(70)

	CachedStyles.Separator100 = strings.Repeat("─", 100)
	CachedStyles.Separator80 = strings.Repeat("─", 80)
	CachedStyles.Separator52 = strings.Repeat("─", 52)

	CachedStyles.Spacer10 = strings.Repeat(" ", 10)
	CachedStyles.Spacer20 = strings.Repeat(" ", 20)
	CachedStyles.Spacer30 = strings.Repeat(" ", 30)
}

// Separator returns a separator line of the given width.
// Note: "─" is a multi-byte UTF-8 character, so we use strings.Repeat
// instead of byte slicing to avoid corrupting the output.
func Separator(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat("─", width)
}

// Spacer returns a space string of the given width using cached values.
func Spacer(width int) string {
	switch {
	case width <= 10:
		return CachedStyles.Spacer10[:width]
	case width <= 20:
		return CachedStyles.Spacer20[:width]
	case width <= 30:
		return CachedStyles.Spacer30[:width]
	default:
		return strings.Repeat(" ", width)
	}
}

// StyledSeparator returns a styled separator line.
func StyledSeparator(width int) string {
	return MutedStyle.Render(Separator(width))
}

// ThemedBoxStyle returns a fresh bordered box style using current theme colors.
func ThemedBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)
}

// Current theme ID
var currentThemeID = "neon"

// Color variables - these will be updated by ApplyTheme
var (
	ColorPrimary   color.Color
	ColorSecondary color.Color
	ColorAccent    color.Color
	ColorText      color.Color
	ColorMuted     color.Color
	ColorSuccess   color.Color
	ColorError     color.Color
	ColorWarning   color.Color
	ColorBorder    color.Color
	ColorBg        color.Color
	ColorCardBg    color.Color
	ColorTestnet   color.Color
	ColorSimulator color.Color
)

// Initialize default theme (neon) on package load
func init() {
	ApplyTheme("neon")
	initCachedStyles()
}

// ApplyTheme updates all color variables and rebuilds styles for the given theme
func ApplyTheme(themeID string) error {
	theme, ok := themes[themeID]
	if !ok {
		return nil // Keep current theme if invalid
	}

	currentThemeID = themeID

	// Update color variables
	ColorPrimary = theme.Primary
	ColorSecondary = theme.Secondary
	ColorAccent = theme.Accent
	ColorText = theme.Text
	ColorMuted = theme.Muted
	ColorSuccess = theme.Success
	ColorError = theme.Error
	ColorWarning = theme.Warning
	ColorBorder = theme.Border
	ColorBg = theme.Bg
	ColorCardBg = theme.CardBg
	ColorTestnet = theme.Testnet
	ColorSimulator = theme.Simulator

	// Rebuild all styles
	rebuildStyles()

	return nil
}

// GetCurrentThemeID returns the currently active theme ID
func GetCurrentThemeID() string {
	return currentThemeID
}

// GetThemeName returns the display name for a theme ID
func GetThemeName(themeID string) string {
	if theme, ok := themes[themeID]; ok {
		return theme.Name
	}
	return "Unknown"
}

// NormalizeTheme returns a valid theme ID, falling back to "neon"
func NormalizeTheme(themeID string) string {
	if _, ok := themes[themeID]; ok {
		return themeID
	}
	return "neon"
}

// GetThemeIDs returns a list of available theme IDs
func GetThemeIDs() []string {
	ids := make([]string, 0, len(themes))
	for id := range themes {
		ids = append(ids, id)
	}
	return ids
}

// GetThemes returns all available themes
func GetThemes() map[string]Theme {
	return themes
}

// rebuildStyles rebuilds all lipgloss styles with current colors
func rebuildStyles() {
	BaseStyle = lipgloss.NewStyle().Background(ColorBg)

	TitleStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
		Foreground(ColorMuted)

	TextStyle = lipgloss.NewStyle().
		Foreground(ColorText)

	MutedStyle = lipgloss.NewStyle().
		Foreground(ColorMuted)

	AccentStyle = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	SuccessStyle = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(ColorError)

	WarningStyle = lipgloss.NewStyle().
		Foreground(ColorWarning)

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)

	SelectedBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	SelectedRowStyle = lipgloss.NewStyle().
		Background(selectedRowBackground()).
		Foreground(ColorText).
		Bold(true)

	ContainerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Width(Width)

	HeaderStyle = lipgloss.NewStyle().
		Width(Width-2).
		Padding(1, 2)

	ContentStyle = lipgloss.NewStyle().
		Padding(1, 2).
		Width(Width - 2)

	FooterStyle = lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		Width(Width-2).
		Padding(0, 2).
		Foreground(ColorMuted)

	TabStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ColorMuted)

	ActiveTabStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(ColorPrimary).
		Bold(true).
		Underline(true)

	InputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Width(InputWidth)

	FocusedInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(InputWidth)

	DisabledInputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted).
		Foreground(ColorMuted).
		Padding(0, 1).
		Width(InputWidth)

	ButtonStyle = lipgloss.NewStyle().
		Padding(0, 3).
		Background(ColorBorder).
		Foreground(ColorText)

	ActiveButtonStyle = lipgloss.NewStyle().
		Padding(0, 3).
		Background(ColorPrimary).
		Foreground(ColorSecondary).
		Bold(true)

	MenuItemStyle = lipgloss.NewStyle().
		Padding(0, 2)

	// SelectedMenuItemStyle has enhanced contrast for neon-pink theme
	if currentThemeID == "neon-pink" {
		SelectedMenuItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
	} else {
		SelectedMenuItemStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)
	}

	BalanceStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	BalanceLargeStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	TxInStyle = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	TxOutStyle = lipgloss.NewStyle().
		Foreground(ColorError)

	StatusOnline = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	StatusOffline = lipgloss.NewStyle().
		Foreground(ColorError)

	StatusSyncing = lipgloss.NewStyle().
		Foreground(ColorWarning)

	TestnetStyle = lipgloss.NewStyle().
		Foreground(ColorTestnet)

	SimulatorStyle = lipgloss.NewStyle().
		Foreground(ColorSimulator)

	// Rebuild cached styles with new theme colors
	initCachedStyles()
}

// Dimensions
const (
	Width      = 80
	InnerWidth = 76
	InputWidth = 60
	// BalanceGlyph is shown before wallet balance amounts.
	BalanceGlyph = "⬡"
)

// Base styles - these will be initialized by rebuildStyles() via init()
var (
	BaseStyle             lipgloss.Style
	TitleStyle            lipgloss.Style
	SubtitleStyle         lipgloss.Style
	TextStyle             lipgloss.Style
	MutedStyle            lipgloss.Style
	AccentStyle           lipgloss.Style
	SuccessStyle          lipgloss.Style
	ErrorStyle            lipgloss.Style
	WarningStyle          lipgloss.Style
	BoxStyle              lipgloss.Style
	SelectedBoxStyle      lipgloss.Style
	ContainerStyle        lipgloss.Style
	HeaderStyle           lipgloss.Style
	ContentStyle          lipgloss.Style
	FooterStyle           lipgloss.Style
	TabStyle              lipgloss.Style
	ActiveTabStyle        lipgloss.Style
	InputStyle            lipgloss.Style
	FocusedInputStyle     lipgloss.Style
	DisabledInputStyle    lipgloss.Style
	ButtonStyle           lipgloss.Style
	ActiveButtonStyle     lipgloss.Style
	MenuItemStyle         lipgloss.Style
	SelectedMenuItemStyle lipgloss.Style
	SelectedRowStyle      lipgloss.Style
	BalanceStyle          lipgloss.Style
	BalanceLargeStyle     lipgloss.Style
	TxInStyle             lipgloss.Style
	TxOutStyle            lipgloss.Style
	StatusOnline          lipgloss.Style
	StatusOffline         lipgloss.Style
	StatusSyncing         lipgloss.Style
	TestnetStyle          lipgloss.Style
	SimulatorStyle        lipgloss.Style
)

// Logo returns the DERO ASCII art logo
func Logo() string {
	logo := `██████╗ ███████╗██████╗  ██████╗
██╔══██╗██╔════╝██╔══██╗██╔═══██╗
██║  ██║█████╗  ██████╔╝██║   ██║
██║  ██║██╔══╝  ██╔══██╗██║   ██║
██████╔╝███████╗██║  ██║╚██████╔╝
╚═════╝ ╚══════╝╚═╝  ╚═╝ ╚═════╝ `
	return TitleStyle.Render(logo)
}

func FormatDERO(atomic uint64) string {
	whole := atomic / 100000
	frac := atomic % 100000
	return lipgloss.NewStyle().Render(
		BalanceStyle.Render(BalanceGlyph+" ") +
			BalanceLargeStyle.Render(formatWithCommas(whole)) +
			MutedStyle.Render(".") +
			TextStyle.Render(padLeft(frac, 5)) +
			MutedStyle.Render(" DERO"),
	)
}

func formatWithCommas(n uint64) string {
	raw := strconv.FormatUint(n, 10)
	if raw == "0" {
		return "0"
	}

	commaCount := (len(raw) - 1) / 3
	var b strings.Builder
	b.Grow(len(raw) + commaCount)

	firstGroup := len(raw) % 3
	if firstGroup == 0 {
		firstGroup = 3
	}
	b.WriteString(raw[:firstGroup])
	for i := firstGroup; i < len(raw); i += 3 {
		b.WriteByte(',')
		b.WriteString(raw[i : i+3])
	}

	return b.String()
}

func uintToStr(n uint64) string {
	return strconv.FormatUint(n, 10)
}

func padLeft(n uint64, width int) string {
	s := uintToStr(n)
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}

// UintToStr converts a uint64 to string.
func UintToStr(n uint64) string {
	return uintToStr(n)
}

// IntToStr converts an int64 to string
func IntToStr(n int64) string {
	return strconv.FormatInt(n, 10)
}

func selectedRowBackground() color.Color {
	switch currentThemeID {
	case "neon":
		return lipgloss.Color("#24123D")
	case "matrix":
		return lipgloss.Color("#00220D")
	case "amber-crt":
		return lipgloss.Color("#352300")
	case "solarized-dark":
		return lipgloss.Color("#0A3541")
	case "crimson-dark":
		return lipgloss.Color("#3A0C14")
	case "neon-pink":
		return lipgloss.Color("#2A0C20")
	case "gruvbox-dark":
		return lipgloss.Color("#4A3A28")
	default:
		return ColorCardBg
	}
}

// SelectedRowBackground returns the current theme selected-row background color.
func SelectedRowBackground() color.Color {
	return selectedRowBackground()
}

// PadLeftInt pads an integer to the specified width with leading zeros
func PadLeftInt(n int64, width int) string {
	s := IntToStr(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}
