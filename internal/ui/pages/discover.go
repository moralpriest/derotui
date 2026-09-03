// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/deroproject/dero-wallet-cli/internal/ui/components"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

const (
	discColName = 56
	discColRate = 12
	discInner   = discColName + discColRate
	discVisible = 10
)

// Sort modes for catalog listings, cycled with [S].
var discoverSortModes = []string{"A-Z", "Recent", "Rating", "SCID"}

var (
	discoverUpKeys     = key.NewBinding(key.WithKeys("up", "k"))
	discoverDownKeys   = key.NewBinding(key.WithKeys("down", "j"))
	discoverLeftKeys   = key.NewBinding(key.WithKeys("left", "h"))
	discoverRightKeys  = key.NewBinding(key.WithKeys("right", "l"))
	discoverFilterKey  = key.NewBinding(key.WithKeys("f"))
	discoverSortKey    = key.NewBinding(key.WithKeys("s"))
	discoverOrderKey   = key.NewBinding(key.WithKeys("o"))
	discoverEnterKeys  = key.NewBinding(key.WithKeys("enter"))
	discoverInfoKeys   = key.NewBinding(key.WithKeys("i", "I"))
	discoverCopySCID   = key.NewBinding(key.WithKeys("c", "C"))
	discoverLikeKey    = key.NewBinding(key.WithKeys("u", "U"))
	discoverDislikeKey = key.NewBinding(key.WithKeys("d", "D"))
	discoverConfirmYes = key.NewBinding(key.WithKeys("y", "Y"))
	discoverConfirmNo  = key.NewBinding(key.WithKeys("n", "N"))
	discoverTab1       = key.NewBinding(key.WithKeys("1"))
	discoverTab2       = key.NewBinding(key.WithKeys("2"))
	discoverTab3       = key.NewBinding(key.WithKeys("3"))
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
	tab          int
	sort         int // index into discoverSortModes
	descending   bool
	tela         []wallet.CatalogEntry
	nft          []wallet.CatalogEntry
	nfa          []wallet.CatalogEntry
	cursor       int
	offset       int
	classifying  bool
	needWallet   bool
	probing      bool
	cancelled    bool
	filtering    bool
	filter       components.InputModel
	detail       bool
	flash        string
	flashOK      bool
	launching    bool
	wantLaunch   bool
	launchSCID   string
	cancelLaunch *atomic.Bool
	votePending  bool
	voteLike     bool
	voteSCID     string
	voting       bool
	wantVote     bool
	ratedSCIDs   map[string]bool
	width        int
	height       int
}

func NewDiscover() DiscoverModel {
	m := DiscoverModel{descending: false, ratedSCIDs: make(map[string]bool)} // A-Z default: ascending
	m.filter = components.NewInput("", "filter name / durl / scid", false)
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

func (m *DiscoverModel) cycleSort() {
	m.sort = (m.sort + 1) % len(discoverSortModes)
	// Sensible default direction per mode (matches Engram conventions):
	// A-Z ascending, Recent newest-first, Rating best-first, SCID ascending.
	m.descending = discoverSortModes[m.sort] == "Recent" || discoverSortModes[m.sort] == "Rating"
	m.cursor = 0
	m.offset = 0
}

func (m *DiscoverModel) toggleOrder() {
	m.descending = !m.descending
	m.cursor = 0
	m.offset = 0
}

func (m *DiscoverModel) setTab(i int) {
	if i == m.tab {
		return
	}
	m.tab = i
	m.cursor = 0
	m.offset = 0
}

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
	if q := strings.ToLower(strings.TrimSpace(m.filter.Value())); q != "" {
		out := make([]wallet.CatalogEntry, 0, len(src))
		for _, e := range src {
			if strings.Contains(strings.ToLower(e.Name), q) ||
				strings.Contains(strings.ToLower(e.SCID), q) ||
				strings.Contains(strings.ToLower(e.DURL), q) ||
				strings.Contains(strings.ToLower(e.Desc), q) {
				out = append(out, e)
			}
		}
		src = out
	}
	if len(src) < 2 {
		return src
	}
	sorted := make([]wallet.CatalogEntry, len(src))
	copy(sorted, src)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		switch discoverSortModes[m.sort] {
		case "SCID":
			if a.SCID != b.SCID {
				if m.descending {
					return a.SCID > b.SCID
				}
				return a.SCID < b.SCID
			}
		case "Recent":
			if a.InstallHeight != b.InstallHeight {
				if m.descending {
					return a.InstallHeight > b.InstallHeight
				}
				return a.InstallHeight < b.InstallHeight
			}
		case "Rating":
			if a.AvgRating != b.AvgRating {
				if m.descending {
					return a.AvgRating > b.AvgRating
				}
				return a.AvgRating < b.AvgRating
			}
		default: // A-Z
			aName, bName := discSortName(a), discSortName(b)
			if aName != bName {
				if m.descending {
					return aName > bName
				}
				return aName < bName
			}
		}
		// Stable tiebreak: name, then SCID, regardless of direction.
		if an, bn := discSortName(a), discSortName(b); an != bn {
			return an < bn
		}
		return a.SCID < b.SCID
	})
	return sorted
}

func discSortName(e wallet.CatalogEntry) string {
	if e.Name != "" {
		return strings.ToLower(e.Name)
	}
	return strings.ToLower(e.DURL)
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

// VisibleSCIDs returns the SCIDs on the current page of the active tab —
// the fetch set for background rating enrichment.
func (m DiscoverModel) VisibleSCIDs() []string {
	rows := m.rows()
	end := m.offset + discVisible
	if end > len(rows) {
		end = len(rows)
	}
	out := make([]string, 0, end-m.offset)
	for i := m.offset; i < end; i++ {
		if rows[i].SCID != "" {
			out = append(out, rows[i].SCID)
		}
	}
	return out
}

// ApplyRatings merges background-fetched rating data into the tab lists.
func (m *DiscoverModel) ApplyRatings(ratings map[string]wallet.CatalogEntry) {
	if len(ratings) == 0 {
		return
	}
	apply := func(s []wallet.CatalogEntry) {
		for i := range s {
			if r, ok := ratings[strings.ToLower(s[i].SCID)]; ok {
				s[i].AvgRating = r.AvgRating
				s[i].Likes = r.Likes
				s[i].Dislikes = r.Dislikes
			}
		}
	}
	apply(m.tela)
	apply(m.nft)
	apply(m.nfa)
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

// DetailOpen reports whether the full-info popup is currently shown.
func (m DiscoverModel) DetailOpen() bool { return m.detail }

func (m DiscoverModel) WantLaunch() (scid string, cancel *atomic.Bool, ok bool) {
	if !m.wantLaunch {
		return "", nil, false
	}
	return m.launchSCID, m.cancelLaunch, true
}

// WantVote returns the selected TELA vote after the user confirms it.
func (m DiscoverModel) WantVote() (scid string, like bool, ok bool) {
	if !m.wantVote {
		return "", false, false
	}
	return m.voteSCID, m.voteLike, true
}

func (m *DiscoverModel) ResetActions() {
	m.wantLaunch = false
	m.wantVote = false
}

// HasVoted reports whether this session has already submitted a vote for SCID.
func (m DiscoverModel) HasVoted(scid string) bool {
	return m.ratedSCIDs[strings.ToLower(strings.TrimSpace(scid))]
}

// MarkVoted prevents a second submission before the indexer sees the vote.
func (m *DiscoverModel) MarkVoted(scid string) {
	if m.ratedSCIDs == nil {
		m.ratedSCIDs = make(map[string]bool)
	}
	m.ratedSCIDs[strings.ToLower(strings.TrimSpace(scid))] = true
}

// SetVoteResult finishes an asynchronous vote submission.
func (m *DiscoverModel) SetVoteResult(txID, err string) {
	m.voting = false
	m.wantVote = false
	if err != "" {
		m.flash = err
		m.flashOK = false
		return
	}
	label := "Like"
	if !m.voteLike {
		label = "Dislike"
	}
	m.flash = label + " submitted"
	m.flashOK = true
	if txID != "" {
		shortID := txID
		if len(shortID) > 12 {
			shortID = shortID[:12] + "..."
		}
		m.flash += " (TX " + shortID + ")"
	}
	m.voteSCID = ""
}

func (m *DiscoverModel) SetLaunchResult(link, err string) {
	m.launching = false
	m.wantLaunch = false
	if link != "" {
		m.flash = "Opened " + link
		m.flashOK = true
		if err != "" {
			m.flash += " (" + err + ")"
			m.flashOK = false
		}
		return
	}
	if err != "" {
		m.flash = err
		m.flashOK = false
	}
}

func (m *DiscoverModel) beginVote(like bool) {
	if m.tab != 0 || m.voting || m.votePending {
		return
	}
	rows := m.rows()
	if len(rows) == 0 || m.cursor < 0 || m.cursor >= len(rows) || rows[m.cursor].SCID == "" {
		return
	}
	scid := rows[m.cursor].SCID
	if m.HasVoted(scid) {
		return
	}
	m.votePending = true
	m.voteLike = like
	m.voteSCID = scid
}

func (m *DiscoverModel) beginLaunch() {
	if m.tab != 0 {
		if rows := m.rows(); len(rows) > 0 && m.cursor < len(rows) {
			m.detail = true
		}
		return
	}
	if m.launching {
		return
	}
	rows := m.rows()
	if len(rows) == 0 || m.cursor >= len(rows) {
		return
	}
	e := rows[m.cursor]
	m.wantLaunch = true
	m.launchSCID = e.SCID
	m.launching = true
	m.cancelLaunch = new(atomic.Bool)
	name := e.Name
	if name == "" {
		name = e.DURL
	}
	m.flash = "Opening " + name + "…"
	m.flashOK = true
}

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
		if m.votePending {
			switch {
			case key.Matches(msg, pageEscKeys), key.Matches(msg, discoverConfirmNo):
				m.votePending = false
				m.voteSCID = ""
			case key.Matches(msg, discoverConfirmYes):
				m.votePending = false
				m.voting = true
				m.wantVote = true
				m.flash = "Submitting vote..."
				m.flashOK = true
			}
			return m, nil
		}
		if m.voting {
			return m, nil
		}
		switch {
		case key.Matches(msg, pageEscKeys):
			if m.votePending {
				m.votePending = false
				m.voteSCID = ""
				return m, nil
			}
			if m.launching {
				if m.cancelLaunch != nil {
					m.cancelLaunch.Store(true)
				}
				m.launching = false
				m.wantLaunch = false
				m.flash = "Launch cancelled"
				m.flashOK = false
				return m, nil
			}
			if m.detail {
				m.detail = false
				return m, nil
			}
			m.cancelled = true
			return m, nil
		case key.Matches(msg, discoverLikeKey):
			m.beginVote(true)
			return m, nil
		case key.Matches(msg, discoverDislikeKey):
			m.beginVote(false)
			return m, nil
		case key.Matches(msg, discoverEnterKeys):
			m.beginLaunch()
			return m, nil
		case key.Matches(msg, discoverInfoKeys):
			if rows := m.rows(); len(rows) > 0 && m.cursor < len(rows) {
				m.detail = !m.detail
			}
			return m, nil
		case key.Matches(msg, discoverCopySCID):
			if m.detail {
				if rows := m.rows(); len(rows) > 0 && m.cursor < len(rows) {
					if err := clipboard.WriteAll(rows[m.cursor].SCID); err == nil {
						m.flash = "SCID copied!"
						m.flashOK = true
					} else {
						m.flash = "Copy failed"
						m.flashOK = false
					}
				}
			}
			return m, nil
		case key.Matches(msg, discoverSortKey):
			m.cycleSort()
			return m, nil
		case key.Matches(msg, discoverOrderKey):
			m.toggleOrder()
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
	} else if m.detail && m.cursor < len(rows) {
		b.WriteString(m.discDetail(rows[m.cursor]))
		b.WriteString("\n")
	} else {
		b.WriteString(m.renderTable(rows))
		b.WriteString("\n")
	}
	if m.votePending {
		voteWord := "dislike"
		if m.voteLike {
			voteWord = "like"
		}
		b.WriteString(styles.WarningStyle.Render(fmt.Sprintf("Submit %s for this TELA app? Costs %s", voteWord, "0.001 DERO")))
		b.WriteString("\n")
	}
	if m.flash != "" {
		st := styles.ErrorStyle
		if m.flashOK {
			st = styles.SuccessStyle
		}
		b.WriteString(st.Render(m.flash))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	order := "↓"
	if !m.descending {
		order = "↑"
	}
	// Compact hint style (matches txdetails) so the footer fits one line in
	// the 72-char box interior. n/m counter dropped to make room for F Filter.
	footer := "Tab • S Sort " + discoverSortModes[m.sort] + order + " • O Ord • F Filter"
	if m.tab == 0 {
		footer += " • Ent Launch • I Info • U Like • D Dislike"
	} else {
		footer += " • Ent • I Info"
	}
	if m.votePending {
		footer += " • Y Confirm • N Cancel"
	}
	footer += " • Esc"
	b.WriteString(styles.MutedStyle.Render(footer))
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
		st.Width(discColRate).Render("Rating"),
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
	rating := discRatingCell(e)
	if selected {
		st := lipgloss.NewStyle().
			Background(styles.ColorPrimary).
			Foreground(styles.ColorText).
			Bold(true)
		return lipgloss.JoinHorizontal(lipgloss.Left,
			st.Width(discColName).Render(name),
			st.Width(discColRate).Render(rating),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(discColName).Foreground(styles.ColorText).Render(name),
		lipgloss.NewStyle().Width(discColRate).Render(rating),
	)
}

// discStars converts the 0-10 average into a 5-star glyph row (one star per
// 2 points, rounded to nearest). Unrated entries render a bare dash.
func discStars(e wallet.CatalogEntry) string {
	if e.AvgRating <= 0 {
		return "—"
	}
	filled := int(math.Round(e.AvgRating / 2))
	if filled < 0 {
		filled = 0
	}
	if filled > 5 {
		filled = 5
	}
	return strings.Repeat("★", filled) + strings.Repeat("☆", 5-filled)
}

// discRatingCell renders the star row only (numeric average lives in the
// detail popup), colored by Engram's rating tiers: purple ≥9, green ≥7,
// yellow ≥5, red otherwise.
func discRatingCell(e wallet.CatalogEntry) string {
	_, tier := e.RatingDisplay()
	if tier == wallet.RatingTierNone {
		return lipgloss.NewStyle().Foreground(styles.ColorMuted).Render("—")
	}
	var fg color.Color
	switch tier {
	case wallet.RatingTierTop:
		fg = styles.ColorPrimary
	case wallet.RatingTierGood:
		fg = styles.ColorSuccess
	case wallet.RatingTierMid:
		fg = styles.ColorWarning
	default:
		fg = styles.ColorError
	}
	return lipgloss.NewStyle().Foreground(fg).Bold(true).Render(discStars(e))
}

// ratingDetailText formats the popup's Rating row: stars, average, and
// like/dislike counts, e.g. "★★★★☆ 7.5 / 10  (12 likes, 3 dislikes)".
func ratingDetailText(e wallet.CatalogEntry) string {
	label, tier := e.RatingDisplay()
	if tier == wallet.RatingTierNone {
		return ""
	}
	return fmt.Sprintf("%s %s / 10  (%d likes, %d dislikes)", discStars(e), label, e.Likes, e.Dislikes)
}

// discClipSCID shortens a 64-hex SCID to head8...tail8 so it fits the
// detail popup's 64-char interior on one line. Short SCIDs pass through.
func discClipSCID(scid string) string {
	if len(scid) <= 20 {
		return scid
	}
	return scid[:8] + "..." + scid[len(scid)-8:]
}

// discDetail renders the full-info popup for the selected entry.
func (m DiscoverModel) discDetail(e wallet.CatalogEntry) string {
	row := func(k, v string) string {
		if v == "" {
			v = "—"
		}
		key := styles.MutedStyle.Width(16).Render(k)
		val := styles.TextStyle.Render(v)
		return key + val
	}
	lines := []string{
		styles.TitleStyle.Render(e.Name),
		"",
		row("Description", e.Desc),
		row("Rating", ratingDetailText(e)),
		row("dURL", e.DURL),
		row("Class", e.Class),
		row("Version", e.Version),
	}
	if len(e.Tags) > 0 {
		lines = append(lines, row("Tags", strings.Join(e.Tags, ", ")))
	}
	if e.InstallHeight > 0 {
		lines = append(lines, row("Installed at", fmt.Sprintf("height %d", e.InstallHeight)))
	}
	lines = append(lines,
		row("SCID", discClipSCID(e.SCID)),
		"",
	)
	hint := "I Close  C Copy SCID"
	if m.tab == 0 {
		hint = "Ent Launch  I Close  C Copy SCID  U Like  D Dislike"
	}
	lines = append(lines, styles.MutedStyle.Render(hint))
	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 2).
		Width(discInner - 4).
		Render(body)
	return box
}
