// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

const (
	discColName = 42
	discColSCID = 19
	discInner   = discColName + discColSCID
	discVisible = 10
)

var (
	discoverUpKeys    = key.NewBinding(key.WithKeys("up", "k"))
	discoverDownKeys  = key.NewBinding(key.WithKeys("down", "j"))
	discoverLeftKeys  = key.NewBinding(key.WithKeys("left", "h"))
	discoverRightKeys = key.NewBinding(key.WithKeys("right", "l"))
	discoverFilterKey = key.NewBinding(key.WithKeys("f"))
	discoverTab1      = key.NewBinding(key.WithKeys("1"))
	discoverTab2      = key.NewBinding(key.WithKeys("2"))
	discoverTab3      = key.NewBinding(key.WithKeys("3"))
)

var discoverTabs = []struct {
	label string
	class string
}{
	{"TELA", "TELA-INDEX-1"},
	{"NFT", "G45-NFT"},
	{"NFA", "NFA"},
}

type DiscoverModel struct {
	tab         int
	tela        []wallet.CatalogEntry
	nft         []wallet.CatalogEntry
	nfa         []wallet.CatalogEntry
	cursor      int
	offset      int
	classifying bool
	needWallet  bool
	probing     bool
	cancelled   bool
	filtering   bool
	filter      components.InputModel
	width       int
	height      int
}

func NewDiscover() DiscoverModel {
	m := DiscoverModel{}
	m.filter = components.NewInput("", "filter name / scid", false)
	m.filter.SetCharLimit(64)
	return m
}

func keepCatalogNames(old, neu []wallet.CatalogEntry) []wallet.CatalogEntry {
	if len(old) == 0 || len(neu) == 0 {
		return neu
	}
	by := make(map[string]string, len(old))
	for _, e := range old {
		if e.Name != "" {
			by[e.SCID] = e.Name
		}
	}
	for i := range neu {
		if neu[i].Name == "" {
			neu[i].Name = by[neu[i].SCID]
		}
	}
	return neu
}

func (m *DiscoverModel) SetCatalog(tela, nft, nfa []wallet.CatalogEntry, classifying bool) {
	m.tela = keepCatalogNames(m.tela, tela)
	m.nft = keepCatalogNames(m.nft, nft)
	m.nfa = keepCatalogNames(m.nfa, nfa)
	m.classifying = classifying
	m.clampCursor()
}

func (m *DiscoverModel) SetTela(tela []wallet.CatalogEntry, classifying, needWallet bool) {
	m.tela = keepCatalogNames(m.tela, tela)
	m.classifying = classifying
	m.needWallet = needWallet
	m.clampCursor()
}

func (m *DiscoverModel) SetOwned(nft, nfa []wallet.CatalogEntry) {
	m.nft = keepCatalogNames(m.nft, nft)
	m.nfa = keepCatalogNames(m.nfa, nfa)
	m.clampCursor()
}

func (m *DiscoverModel) SetProbing(v bool) { m.probing = v }

func (m DiscoverModel) Classifying() bool { return m.classifying }

func (m *DiscoverModel) clampCursor() {
	if m.cursor >= len(m.rows()) && len(m.rows()) > 0 {
		m.cursor = len(m.rows()) - 1
	}
	if len(m.rows()) == 0 {
		m.cursor = 0
		m.offset = 0
	}
	m.clampOffset()
}

func (m DiscoverModel) rows() []wallet.CatalogEntry {
	var src []wallet.CatalogEntry
	switch m.tab {
	case 1:
		src = m.nft
	case 2:
		src = m.nfa
	default:
		src = m.tela
	}
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if q == "" {
		return src
	}
	out := make([]wallet.CatalogEntry, 0)
	for _, e := range src {
		if strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.SCID), q) ||
			strings.Contains(strings.ToLower(e.DURL), q) {
			out = append(out, e)
		}
	}
	return out
}

func (m *DiscoverModel) clampOffset() {
	n := len(m.rows())
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+discVisible {
		m.offset = m.cursor - discVisible + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	if n == 0 {
		m.offset = 0
	}
}

func (m DiscoverModel) UnnamedVisible() []string {
	rows := m.rows()
	end := m.offset + discVisible
	if end > len(rows) {
		end = len(rows)
	}
	var out []string
	for i := m.offset; i < end; i++ {
		if rows[i].Name == "" && rows[i].SCID != "" {
			out = append(out, rows[i].SCID)
		}
	}
	return out
}

func (m *DiscoverModel) ApplyNames(names map[string]string) {
	if len(names) == 0 {
		return
	}
	apply := func(s []wallet.CatalogEntry) {
		for i := range s {
			if n := names[s[i].SCID]; n != "" {
				s[i].Name = n
			}
		}
	}
	apply(m.tela)
	apply(m.nft)
	apply(m.nfa)
}

func (m DiscoverModel) Cancelled() bool { return m.cancelled }

func (m *DiscoverModel) ClearCancelled() { m.cancelled = false }

func (m DiscoverModel) Init() tea.Cmd { return nil }

func (m DiscoverModel) Update(msg tea.Msg) (DiscoverModel, tea.Cmd) {
	var cmd tea.Cmd
	if m.filtering {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if key.Matches(msg, pageEscKeys) {
				m.filtering = false
				m.filter.Blur()
				m.cursor = 0
				m.offset = 0
				return m, nil
			}
			if key.Matches(msg, pageEnterKeys) {
				m.filtering = false
				m.filter.Blur()
				m.cursor = 0
				m.offset = 0
				return m, nil
			}
		}
		m.filter, cmd = m.filter.Update(msg)
		m.cursor = 0
		m.offset = 0
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, pageEscKeys):
			m.cancelled = true
			return m, nil
		case key.Matches(msg, discoverFilterKey):
			m.filtering = true
			return m, m.filter.Focus()
		case key.Matches(msg, discoverTab1):
			m.setTab(0)
		case key.Matches(msg, discoverTab2):
			m.setTab(1)
		case key.Matches(msg, discoverTab3):
			m.setTab(2)
		case key.Matches(msg, pageTabKeys), key.Matches(msg, discoverRightKeys):
			m.setTab((m.tab + 1) % len(discoverTabs))
		case key.Matches(msg, pageShiftTabKeys), key.Matches(msg, discoverLeftKeys):
			m.setTab((m.tab + len(discoverTabs) - 1) % len(discoverTabs))
		case key.Matches(msg, discoverUpKeys):
			if m.cursor > 0 {
				m.cursor--
				m.clampOffset()
			}
		case key.Matches(msg, discoverDownKeys):
			if m.cursor < len(m.rows())-1 {
				m.cursor++
				m.clampOffset()
			}
		}
	}
	return m, nil
}

func (m *DiscoverModel) setTab(i int) {
	if i == m.tab {
		return
	}
	m.tab = i
	m.cursor = 0
	m.offset = 0
}

func (m DiscoverModel) View() string {
	var b strings.Builder
	b.WriteString(m.renderTabs())
	b.WriteString("\n")
	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.filter.View())
		b.WriteString("\n")
	}
	rows := m.rows()
	if len(rows) == 0 {
		b.WriteString(m.emptyMsg())
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderTable(rows))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.MutedStyle.Render("[1-3]/Tab tabs  [F]ilter  ↑↓  [Esc] Back"))
	if len(rows) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.MutedStyle.Render(fmt.Sprintf("%d / %d", m.cursor+1, len(rows))))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, discTitle("Discover"), "", b.String())
	return styles.ThemedBoxStyle().
		Width(styles.Width).
		Align(lipgloss.Left).
		Padding(1, 4).
		Render(content)
}

func (m DiscoverModel) emptyMsg() string {
	if m.classifying {
		return styles.WarningStyle.Render("Classifying…")
	}
	if m.tab != 0 && m.needWallet {
		return styles.MutedStyle.Render("Open a wallet to see what you own")
	}
	if m.tab != 0 && m.probing {
		return styles.WarningStyle.Render("Checking holdings…")
	}
	if m.tab != 0 {
		return styles.MutedStyle.Render("None owned")
	}
	return styles.MutedStyle.Render("No entries in this catalog")
}

func discTitle(text string) string {
	return styles.TitleStyle.Width(discInner).Align(lipgloss.Center).Render(text)
}

func (m DiscoverModel) renderTabs() string {
	var parts []string
	for i, t := range discoverTabs {
		label := " " + t.label + " "
		if i == m.tab {
			parts = append(parts, lipgloss.NewStyle().
				Background(styles.ColorPrimary).
				Foreground(styles.ColorText).
				Bold(true).
				Render(label))
		} else {
			parts = append(parts, styles.MutedStyle.Render(label))
		}
	}
	return strings.Join(parts, " ")
}

func (m DiscoverModel) renderTable(rows []wallet.CatalogEntry) string {
	st := lipgloss.NewStyle().Bold(true).Foreground(styles.ColorMuted)
	header := lipgloss.JoinHorizontal(lipgloss.Left,
		st.Width(discColName).Render("Name"),
		st.Width(discColSCID).Render("SCID"),
	)
	sep := styles.StyledSeparator(discInner)
	end := m.offset + discVisible
	if end > len(rows) {
		end = len(rows)
	}
	var lines []string
	for i := m.offset; i < end; i++ {
		lines = append(lines, discRow(rows[i], i == m.cursor))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, sep, strings.Join(lines, "\n"))
}

func discRow(e wallet.CatalogEntry, selected bool) string {
	name := e.Name
	if name == "" {
		name = "—"
	}
	name = clip(name, discColName)
	scid := safeSCIDLabel(e.SCID)
	if selected {
		st := lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorText).
			Bold(true)
		return lipgloss.JoinHorizontal(lipgloss.Left,
			st.Width(discColName).Render(name),
			st.Width(discColSCID).Render(scid),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(discColName).Foreground(styles.ColorText).Render(name),
		lipgloss.NewStyle().Width(discColSCID).Foreground(styles.ColorMuted).Render(scid),
	)
}
