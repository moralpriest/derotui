// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

type LogoModel struct {
	cancelled bool
}

func NewLogo() LogoModel { return LogoModel{} }

func (m LogoModel) Init() tea.Cmd { return nil }

func (m LogoModel) Update(msg tea.Msg) (LogoModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(keyMsg, pageEscKeys) || key.Matches(keyMsg, pageEnterKeys) {
			m.cancelled = true
		}
	}
	return m, nil
}

func (m LogoModel) View() string {
	return lipgloss.JoinVertical(lipgloss.Center,
		styles.HexLogo(),
		"",
		styles.MutedStyle.Render("[Esc] Back"),
	)
}

func (m LogoModel) Cancelled() bool { return m.cancelled }

func (m *LogoModel) ClearCancelled() { m.cancelled = false }

func (m LogoModel) HandleMouse(msg tea.MouseClickMsg, _, _ int) LogoModel {
	if msg.Button == tea.MouseLeft {
		m.cancelled = true
	}
	return m
}
