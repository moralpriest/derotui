// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// PaletteModel is a global slash-command palette that can be overlaid on any
// page. It reuses the welcome command list and selection style so commands
// behave identically everywhere in the app.
type PaletteModel struct {
	input         textinput.Model
	commands      []Command
	Filtered      []Command
	Selected      int
	open          bool
	action        WelcomeAction
	walletOpen    bool
	inRestoreMenu bool
	restoreSel    int
	inThemesMenu  bool
	themesSel     int
	selectedTheme string
}

// NewPalette creates a command palette.
func NewPalette() PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.CharLimit = 64
	ti.SetWidth(38)
	ti.Focus()
	return PaletteModel{
		input:    ti,
		Filtered: []Command{},
	}
}

// IsOpen reports whether the palette is currently shown.
func (p PaletteModel) IsOpen() bool { return p.open }

// Action returns the selected action.
func (p PaletteModel) Action() WelcomeAction { return p.action }

// SelectedTheme returns the previewed/selected theme id.
func (p PaletteModel) SelectedTheme() string { return p.selectedTheme }

// ResetAction clears the pending action.
func (p *PaletteModel) ResetAction() { p.action = ActionNone }

// Open shows the palette, building the command list for the current state.
func (p *PaletteModel) Open(walletOpen bool) {
	p.walletOpen = walletOpen
	p.commands = buildPaletteCommands(walletOpen)
	p.input.SetValue("/")
	p.input.SetCursor(1)
	p.Filtered = filterPaletteCommands(p.commands, "/")
	p.Selected = 0
	p.inRestoreMenu = false
	p.inThemesMenu = false
	p.open = true
	p.action = ActionNone
}

// Close hides the palette but preserves the selected action so the caller
// can dispatch it. Esc path explicitly clears the action before calling Close.
func (p *PaletteModel) Close() {
	p.open = false
	p.Filtered = []Command{}
	p.Selected = 0
	p.input.SetValue("")
	p.input.Reset()
}

// buildPaletteCommands returns the context-aware command list.
func buildPaletteCommands(walletOpen bool) []Command {
	commands := []Command{
		{Name: "/open", Description: "Open a wallet", Action: ActionOpen},
	}
	if walletOpen {
		commands = append(commands, Command{Name: "/close", Description: "Close current wallet", Action: ActionCloseWallet})
		commands = append(commands, Command{Name: "/tokens", Description: "Manage tokens", Action: ActionTokens})
	} else {
		commands = append(commands,
			Command{Name: "/create", Description: "Create a new wallet", Action: ActionCreate},
			Command{Name: "/restore", Description: "Restore a wallet", Action: ActionRestore},
		)
	}
	commands = append(commands,
		Command{Name: "/discover", Description: "Browse TELA, NFTs, NFA", Action: ActionDiscover},
		Command{Name: "/miner", Description: "Start embedded miner", Action: ActionMiner},
		Command{Name: "/daemon", Description: "Manage local daemon", Action: ActionDaemon},
		Command{Name: "/connect", Description: "Connect to a daemon", Action: ActionConnectDaemon},
		Command{Name: "/themes", Description: "Change color theme", Action: ActionThemes},
		Command{Name: "/debug", Description: "Open debug console", Action: ActionDebug},
		Command{Name: "/logo", Description: "Preview hex logo", Action: ActionLogo},
		Command{Name: "/exit", Description: "Exit the application", Action: ActionExit},
	)
	return commands
}

// filterPaletteCommands returns commands that match the input.
func filterPaletteCommands(commands []Command, input string) []Command {
	var result []Command
	input = strings.ToLower(input)
	for _, c := range commands {
		if strings.HasPrefix(strings.ToLower(c.Name), input) {
			result = append(result, c)
		}
	}
	return result
}

// Update handles palette key events. The caller only forwards messages while
// the palette is open.
func (p PaletteModel) Update(msg tea.Msg) (PaletteModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Esc closes submenus first, then the palette itself.
		if key.Matches(msg, pageEscKeys) {
			if p.inRestoreMenu {
				p.inRestoreMenu = false
				p.restoreSel = 0
				return p, nil
			}
			if p.inThemesMenu {
				p.inThemesMenu = false
				p.themesSel = 0
				return p, nil
			}
			p.action = ActionNone
			p.Close()
			return p, nil
		}

		if p.inRestoreMenu {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if p.restoreSel > 0 {
					p.restoreSel--
				}
			case key.Matches(msg, welcomeDownKeys):
				if p.restoreSel < len(p.restoreOptions())-1 {
					p.restoreSel++
				}
			case key.Matches(msg, pageEnterKeys):
				options := p.restoreOptions()
				if p.restoreSel < len(options) {
					p.action = options[p.restoreSel].Action
					p.Close()
				}
			}
			return p, nil
		}

		if p.inThemesMenu {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if p.themesSel > 0 {
					p.themesSel--
				}
				p.previewTheme()
			case key.Matches(msg, welcomeDownKeys):
				if p.themesSel < len(p.themeOptions())-1 {
					p.themesSel++
				}
				p.previewTheme()
			case key.Matches(msg, pageEnterKeys):
				if len(p.themeOptions()) > 0 {
					p.selectedTheme = p.themeOptions()[p.themesSel].Description
					p.action = ActionSetTheme
				}
				p.Close()
			}
			return p, nil
		}

		// Navigate the filtered command list.
		if len(p.Filtered) > 0 {
			switch {
			case key.Matches(msg, welcomeUpKeys):
				if p.Selected > 0 {
					p.Selected--
				}
				return p, nil
			case key.Matches(msg, welcomeDownKeys):
				if p.Selected < len(p.Filtered)-1 {
					p.Selected++
				}
				return p, nil
			case key.Matches(msg, pageTabKeys):
				// Autocomplete selected command
				if p.Selected < len(p.Filtered) {
					p.input.SetValue(p.Filtered[p.Selected].Name)
					p.input.SetCursor(len(p.Filtered[p.Selected].Name))
					p.Filtered = filterPaletteCommands(p.commands, p.input.Value())
				}
				return p, nil
			case key.Matches(msg, pageEnterKeys):
				if p.Selected < len(p.Filtered) {
					selectedAction := p.Filtered[p.Selected].Action
					switch selectedAction {
					case ActionRestore:
						p.inRestoreMenu = true
						p.restoreSel = 0
						return p, nil
					case ActionThemes:
						p.inThemesMenu = true
						p.themesSel = 0
						p.previewTheme()
						return p, nil
					default:
						p.action = selectedAction
						p.Close()
						return p, nil
					}
				}
				return p, nil
			}
		}

		// Otherwise forward to the input field.
		prevValue := p.input.Value()
		p.input, cmd = p.input.Update(msg)
		newValue := p.input.Value()
		if newValue != prevValue {
			if strings.HasPrefix(newValue, "/") {
				p.Filtered = filterPaletteCommands(p.commands, newValue)
				p.Selected = 0
			} else {
				p.Filtered = []Command{}
				p.Selected = 0
			}
		}
	}

	return p, cmd
}

func (p *PaletteModel) previewTheme() {
	options := p.themeOptions()
	if len(options) > 0 && p.themesSel >= 0 && p.themesSel < len(options) {
		p.selectedTheme = options[p.themesSel].Description
		p.action = ActionPreviewTheme
	}
}

func (p PaletteModel) restoreOptions() []Command {
	return []Command{
		{Name: "From Seed", Description: "25-word seed words", Action: ActionRestoreSeed},
		{Name: "From Key", Description: "64 character hex key", Action: ActionRestoreKey},
	}
}

func (p PaletteModel) themeOptions() []Command {
	return buildThemeOptions()
}

// View renders the palette as a compact centered overlay.
func (p PaletteModel) View() string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 1).
		Width(40)
	inputView := inputStyle.Render(p.input.View())

	var menuView string
	var help string

	switch {
	case p.inRestoreMenu:
		menuView = p.renderSubmenu("Restore Wallet", p.restoreOptions(), p.restoreSel)
		help = "↑↓ Navigate • Enter Select • Esc Cancel"
	case p.inThemesMenu:
		menuView = p.renderSubmenu("Select Theme", p.themeOptions(), p.themesSel)
		help = "↑↓ Navigate • Enter Select • Esc Cancel • ✓ Current"
	case len(p.Filtered) > 0:
		menuView = p.renderCommandMenu()
		help = "↑↓ Navigate • Tab Complete • Enter Select • Esc Cancel"
	default:
		help = "Type a command..."
	}

	elements := []string{inputView}
	if menuView != "" {
		elements = append(elements, menuView)
	}
	elements = append(elements, "", styles.MutedStyle.Render(help))

	menu := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Center, elements...))
	return menu
}

// renderCommandMenu renders the filtered slash-command list.
func (p PaletteModel) renderCommandMenu() string {
	filteredLabels := make([]string, 0, len(p.Filtered))
	for _, c := range p.Filtered {
		filteredLabels = append(filteredLabels, c.Name)
	}
	labelWidth := maxLabelWidth(filteredLabels)

	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorBorder).
		Padding(0, 1).
		Width(40)

	var items []string
	items = append(items, styles.TitleStyle.Render("Slash Commands"), "")
	for i, c := range p.Filtered {
		namePadded := padLabel(c.Name, labelWidth)
		if i == p.Selected {
			rowPlain := namePadded + " - " + c.Description
			item := styles.SelectedMenuItemStyle.Render("▸ ") + selectedMenuRow(rowPlain, 36)
			items = append(items, item)
		} else {
			desc := styles.MutedStyle.Render(" - " + c.Description)
			cmdName := styles.MutedStyle.Render(namePadded)
			items = append(items, "  "+cmdName+desc)
		}
	}
	return menuStyle.Render(lipgloss.JoinVertical(lipgloss.Left, items...))
}

// renderSubmenu renders a simple selection submenu (restore / themes).
func (p PaletteModel) renderSubmenu(title string, options []Command, selected int) string {
	labels := make([]string, 0, len(options))
	for _, c := range options {
		labels = append(labels, c.Name)
	}
	labelWidth := maxLabelWidth(labels)

	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 1).
		Width(40)

	currentTheme := config.GetTheme()
	var items []string
	items = append(items, styles.TitleStyle.Render(title), "")
	for i, c := range options {
		namePadded := padLabel(c.Name, labelWidth)
		isCurrent := c.Description == currentTheme
		if i == selected {
			var rowPlain string
			if isCurrent {
				rowPlain = namePadded + " ✓"
			} else {
				rowPlain = namePadded
			}
			item := styles.SelectedMenuItemStyle.Render("▸ ") + selectedMenuRow(rowPlain, 36)
			items = append(items, item)
		} else {
			cmdName := styles.MutedStyle.Render(namePadded)
			var item string
			if isCurrent {
				item = "  " + cmdName + styles.SuccessStyle.Render(" ✓")
			} else {
				item = "  " + cmdName
			}
			items = append(items, item)
		}
	}
	return menuStyle.Render(lipgloss.JoinVertical(lipgloss.Left, items...))
}
