// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"log"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

var (
	xswdAuthLeftKeys        = key.NewBinding(key.WithKeys("left", "h"))
	xswdAuthRightKeys       = key.NewBinding(key.WithKeys("right", "l"))
	xswdAuthAcceptKeys      = key.NewBinding(key.WithKeys("a"))
	xswdAuthRejectKeys      = key.NewBinding(key.WithKeys("r"))
	xswdPermUpKeys          = key.NewBinding(key.WithKeys("up", "k"))
	xswdPermDownKeys        = key.NewBinding(key.WithKeys("down", "j"))
	xswdPermAllowKeys       = key.NewBinding(key.WithKeys("1"))
	xswdPermDenyKeys        = key.NewBinding(key.WithKeys("2"))
	xswdPermAlwaysAllowKeys = key.NewBinding(key.WithKeys("3"))
	xswdPermAlwaysDenyKeys  = key.NewBinding(key.WithKeys("4"))
)

// XSWDAuthModel represents the XSWD app authorization dialog
type XSWDAuthModel struct {
	appName   string
	appDesc   string
	appURL    string
	appID     string
	selected  int // 0=Accept, 1=Reject
	confirmed bool
	accepted  bool // true if user accepted
}

// NewXSWDAuth creates a new XSWD authorization dialog
func NewXSWDAuth(name, description, url, id string) XSWDAuthModel {
	return XSWDAuthModel{
		appName:  name,
		appDesc:  description,
		appURL:   url,
		appID:    id,
		selected: 1, // Default to Reject for safety
	}
}

// Init initializes the model
func (m XSWDAuthModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (m XSWDAuthModel) Update(msg tea.Msg) (XSWDAuthModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, xswdAuthLeftKeys):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, xswdAuthRightKeys):
			if m.selected < 1 {
				m.selected++
			}
		case key.Matches(msg, pageTabKeys):
			m.selected = (m.selected + 1) % 2
		case key.Matches(msg, xswdAuthAcceptKeys):
			m.confirmed = true
			m.accepted = true
			log.Printf("[DEBUG xswd-auth] User accepted connection from %s", m.appName)
		case key.Matches(msg, xswdAuthRejectKeys):
			m.confirmed = true
			m.accepted = false
			log.Printf("[DEBUG xswd-auth] User rejected connection from %s", m.appName)
		case key.Matches(msg, pageEnterKeys):
			m.confirmed = true
			m.accepted = m.selected == 0
			if m.accepted {
				log.Printf("[DEBUG xswd-auth] User accepted connection from %s (via enter)", m.appName)
			} else {
				log.Printf("[DEBUG xswd-auth] User rejected connection from %s (via enter)", m.appName)
			}
		case key.Matches(msg, pageEscKeys):
			m.confirmed = true
			m.accepted = false
			log.Printf("[DEBUG xswd-auth] User rejected connection from %s (via esc)", m.appName)
		}
	}

	return m, nil
}

// View renders the authorization dialog
func (m XSWDAuthModel) View() string {
	contentWidth := 52

	// Helper to center plain text by calculating padding manually
	centerPlain := func(text string, totalWidth int) string {
		visibleLen := len(text)
		if visibleLen >= totalWidth {
			return text
		}
		leftPad := (totalWidth - visibleLen) / 2
		rightPad := totalWidth - visibleLen - leftPad
		return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
	}

	// Title - center the plain text first, then style
	titlePlain := centerPlain("XSWD Connection Request", contentWidth)
	title := styles.TitleStyle.Render(titlePlain)

	// Warning - center the plain text first, then style
	warningPlain := centerPlain("A dApp is requesting access to your wallet", contentWidth)
	warning := styles.WarningStyle.Render(warningPlain)

	// Separator
	sep := styles.StyledSeparator(contentWidth)

	// App details
	nameLabel := styles.MutedStyle.Render("Name:  ")
	nameValue := styles.TextStyle.Bold(true).Render(m.appName)

	descLabel := styles.MutedStyle.Render("About: ")
	// Word-wrap description if too long
	descText := m.appDesc
	if len(descText) > contentWidth-7 {
		descText = descText[:contentWidth-10] + "..."
	}
	descValue := styles.TextStyle.Render(descText)

	urlLabel := styles.MutedStyle.Render("URL:   ")
	urlText := m.appURL
	if urlText == "" {
		urlText = "(none)"
	}
	if len(urlText) > contentWidth-7 {
		urlText = urlText[:contentWidth-10] + "..."
	}
	urlValue := styles.TextStyle.Render(urlText)

	idLabel := styles.MutedStyle.Render("ID:    ")
	idText := m.appID
	if len(idText) > contentWidth-7 {
		idText = idText[:contentWidth-10] + "..."
	}
	idValue := styles.MutedStyle.Render(idText)

	// Buttons
	acceptText := " Accept "
	rejectText := " Reject "

	var acceptBtn, rejectBtn string
	if m.selected == 0 {
		acceptBtn = lipgloss.NewStyle().
			Background(styles.ColorSuccess).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Render(acceptText)
		rejectBtn = lipgloss.NewStyle().
			Foreground(styles.ColorMuted).
			Render(rejectText)
	} else {
		acceptBtn = lipgloss.NewStyle().
			Foreground(styles.ColorMuted).
			Render(acceptText)
		rejectBtn = lipgloss.NewStyle().
			Background(styles.ColorError).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Render(rejectText)
	}

	// Center buttons using plain-space padding based on visible text width.
	buttonsCombined := acceptBtn + "    " + rejectBtn
	buttonVisibleLen := len(acceptText) + 4 + len(rejectText) // visible width only
	leftPad := 0
	rightPad := 0
	if buttonVisibleLen < contentWidth {
		leftPad = (contentWidth - buttonVisibleLen) / 2
		rightPad = contentWidth - buttonVisibleLen - leftPad
	}
	buttonsBlock := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(strings.Repeat(" ", leftPad) + buttonsCombined + strings.Repeat(" ", rightPad))

	// Help - center plain text first
	helpText := "A Accept • R Reject • Tab Switch • Enter Confirm"
	helpPlain := centerPlain(helpText, contentWidth)
	help := styles.MutedStyle.Render(helpPlain)

	// Keep detail rows left-aligned in a fixed-width block for visual centering stability
	infoBlock := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			nameLabel+nameValue,
			descLabel+descValue,
			urlLabel+urlValue,
			idLabel+idValue,
		))

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		warning,
		"",
		sep,
		"",
		infoBlock,
		"",
		sep,
		"",
		buttonsBlock,
		"",
		help,
	)

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(2, 4).
		Render(content)
}

// Confirmed returns true if the user has made a decision
func (m XSWDAuthModel) Confirmed() bool {
	return m.confirmed
}

// Accepted returns true if the user accepted the app
func (m XSWDAuthModel) Accepted() bool {
	return m.accepted
}

// Reset resets the model state
func (m *XSWDAuthModel) Reset() {
	m.confirmed = false
	m.accepted = false
	m.selected = 1
}

// HandleMouse handles mouse events
func (m XSWDAuthModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) XSWDAuthModel {
	if msg.Button != tea.MouseLeft {
		return m
	}

	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - 22) / 2 // Approximate dialog height

	relX := msg.Mouse().X - boxX - 6 // Account for border + padding
	relY := msg.Mouse().Y - boxY - 4 // Account for border + padding

	// Buttons are at approximately relY == 14 (after title, warning, sep, 4 info lines, sep, blank)
	if relY >= 13 && relY <= 15 {
		// Accept button on left side, Reject on right
		if relX >= 0 && relX < 20 {
			m.selected = 0
			m.confirmed = true
			m.accepted = true
		} else if relX >= 20 {
			m.selected = 1
			m.confirmed = true
			m.accepted = false
		}
	}

	return m
}

// --- Permission Request Dialog ---

// XSWDPermModel represents the XSWD permission request dialog
type XSWDPermModel struct {
	appName   string
	method    string
	selected  int // 0=Allow, 1=Deny, 2=AlwaysAllow, 3=AlwaysDeny
	confirmed bool
	// Permission values match xswd.Permission enum:
	// Allow=1, Deny=2, AlwaysAllow=3, AlwaysDeny=4
	result int
}

// NewXSWDPerm creates a new permission request dialog
func NewXSWDPerm(appName, method string) XSWDPermModel {
	return XSWDPermModel{
		appName:  appName,
		method:   method,
		selected: 1, // Default to Deny (index 1)
	}
}

// Init initializes the model
func (m XSWDPermModel) Init() tea.Cmd {
	return nil
}

// Update handles events
func (m XSWDPermModel) Update(msg tea.Msg) (XSWDPermModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, xswdPermUpKeys):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, xswdPermDownKeys):
			if m.selected < 3 {
				m.selected++
			}
		case key.Matches(msg, xswdPermAllowKeys):
			m.selected = 0
			m.confirmed = true
			m.result = 1 // Allow
			log.Printf("[DEBUG xswd-perm] User granted Allow for %s.%s", m.appName, m.method)
		case key.Matches(msg, xswdPermDenyKeys):
			m.selected = 1
			m.confirmed = true
			m.result = 2 // Deny
			log.Printf("[DEBUG xswd-perm] User denied Deny for %s.%s", m.appName, m.method)
		case key.Matches(msg, xswdPermAlwaysAllowKeys):
			m.selected = 2
			m.confirmed = true
			m.result = 3 // AlwaysAllow
			log.Printf("[DEBUG xswd-perm] User granted AlwaysAllow for %s.%s", m.appName, m.method)
		case key.Matches(msg, xswdPermAlwaysDenyKeys):
			m.selected = 3
			m.confirmed = true
			m.result = 4 // AlwaysDeny
			log.Printf("[DEBUG xswd-perm] User denied AlwaysDeny for %s.%s", m.appName, m.method)
		case key.Matches(msg, pageEnterKeys):
			m.confirmed = true
			m.result = m.selected + 1 // Allow=1, Deny=2, AlwaysAllow=3, AlwaysDeny=4
			perms := []string{"", "Allow", "Deny", "AlwaysAllow", "AlwaysDeny"}
			log.Printf("[DEBUG xswd-perm] User granted %s for %s.%s (via enter)", perms[m.result], m.appName, m.method)
		case key.Matches(msg, pageEscKeys):
			m.confirmed = true
			m.result = 2 // Deny
			log.Printf("[DEBUG xswd-perm] User denied Deny for %s.%s (via esc)", m.appName, m.method)
		}
	}

	return m, nil
}

// View renders the permission request dialog
func (m XSWDPermModel) View() string {
	contentWidth := 64

	// Helper to center plain text by calculating padding manually
	centerPlain := func(text string, totalWidth int) string {
		visibleLen := len(text)
		if visibleLen >= totalWidth {
			return text
		}
		leftPad := (totalWidth - visibleLen) / 2
		rightPad := totalWidth - visibleLen - leftPad
		return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
	}

	// Title - center the plain text first, then style
	titlePlain := centerPlain("XSWD Permission Request", contentWidth)
	title := styles.TitleStyle.Render(titlePlain)

	// App + method info
	appLabel := styles.MutedStyle.Render("App:    ")
	appValue := styles.TextStyle.Bold(true).Render(m.appName)

	methodLabel := styles.MutedStyle.Render("Method: ")
	methodValue := lipgloss.NewStyle().
		Foreground(styles.ColorWarning).
		Bold(true).
		Render(m.method)

	sep := styles.StyledSeparator(contentWidth)

	// Permission options
	type permOption struct {
		label string
		desc  string
	}

	options := []permOption{
		{"Allow", "This request only"},
		{"Deny", "This request only"},
		{"Always Allow", "All future requests for this method"},
		{"Always Deny", "All future requests for this method"},
	}

	labels := make([]string, 0, len(options))
	for _, opt := range options {
		labels = append(labels, opt.label)
	}
	labelWidth := maxLabelWidth(labels)

	var optionsView string
	for i, opt := range options {
		num := fmt.Sprintf("[%d]", i+1)
		labelPlain := padLabel(opt.label, labelWidth)
		sepText := styles.MutedStyle.Render(" - ")
		if i == m.selected {
			cursor := styles.SelectedMenuItemStyle.Render("▸ ")
			numStyled := lipgloss.NewStyle().Foreground(styles.ColorPrimary).Render(num)
			label := styles.TextStyle.Bold(true).Render(labelPlain)
			desc := styles.MutedStyle.Render(opt.desc)
			optionsView += cursor + numStyled + " " + label + sepText + desc + "\n"
		} else {
			numStyled := lipgloss.NewStyle().Foreground(styles.ColorBorder).Render(num)
			label := styles.MutedStyle.Render(labelPlain)
			desc := styles.MutedStyle.Render(opt.desc)
			optionsView += "  " + numStyled + " " + label + sepText + desc + "\n"
		}
	}

	// Help - center plain text first
	helpText := "1-4 Select • ↑↓ Navigate • Enter Confirm • Esc Deny"
	helpPlain := centerPlain(helpText, contentWidth)
	help := styles.MutedStyle.Render(helpPlain)

	hintPlain := centerPlain("Esc defaults to Deny", contentWidth)
	hint := styles.WarningStyle.Render(hintPlain)

	metaBlock := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			appLabel+appValue,
			methodLabel+methodValue,
		))

	optionsBlock := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(strings.TrimSuffix(optionsView, "\n"))

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		metaBlock,
		"",
		sep,
		"",
		optionsBlock,
		sep,
		"",
		hint,
		"",
		help,
	)

	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Center).
		Padding(2, 4).
		Render(content)
}

// Confirmed returns true if the user has made a decision
func (m XSWDPermModel) Confirmed() bool {
	return m.confirmed
}

// Result returns the permission result as an int matching xswd.Permission enum
// Allow=1, Deny=2, AlwaysAllow=3, AlwaysDeny=4
func (m XSWDPermModel) Result() int {
	return m.result
}

// Reset resets the model state
func (m *XSWDPermModel) Reset() {
	m.confirmed = false
	m.result = 0
	m.selected = 0
}

// HandleMouse handles mouse events
func (m XSWDPermModel) HandleMouse(msg tea.MouseClickMsg, windowWidth, windowHeight int) XSWDPermModel {
	if msg.Button != tea.MouseLeft {
		return m
	}

	// Calculate centered position
	boxWidth := styles.Width
	boxX := (windowWidth - boxWidth) / 2
	boxY := (windowHeight - styles.SmallBoxHeight) / 2

	relX := msg.Mouse().X - boxX - 6
	relY := msg.Mouse().Y - boxY - 4

	// Options start at approximately relY == 8 (after title, app info, sep, blank)
	optionStartY := 8
	if relX >= 0 && relX < 50 {
		idx := relY - optionStartY
		if idx >= 0 && idx <= 3 {
			m.selected = idx
			m.confirmed = true
			m.result = idx + 1 // Allow=1, Deny=2, AlwaysAllow=3, AlwaysDeny=4
		}
	}

	return m
}
