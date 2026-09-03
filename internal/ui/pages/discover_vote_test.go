// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

func TestDiscoverTelaCatalogFooterHidesVoteHints(t *testing.T) {
	m := NewDiscover()
	m.SetTela([]wallet.CatalogEntry{{
		SCID: strings.Repeat("f", 64), Class: "TELA-INDEX-1", Name: "Catalog app",
	}}, false, false)

	catalogView := stripANSI(m.View())
	for _, hidden := range []string{"U Like", "D Dislike"} {
		if strings.Contains(catalogView, hidden) {
			t.Fatalf("catalog footer should hide %q: %q", hidden, catalogView)
		}
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "i"})
	infoView := stripANSI(m.View())
	for _, visible := range []string{"U Like", "D Dislike"} {
		if !strings.Contains(infoView, visible) {
			t.Fatalf("info footer should keep %q: %q", visible, infoView)
		}
	}
}

func TestDiscoverTelaInfoUsesCompactFooter(t *testing.T) {
	m := NewDiscover()
	m.SetTela([]wallet.CatalogEntry{{
		SCID: strings.Repeat("e", 64), Class: "TELA-INDEX-1", Name: "Info app",
	}}, false, false)
	m, _ = m.Update(tea.KeyPressMsg{Text: "i"})

	view := stripANSI(m.View())
	footer := "Ent Launch • C Copy SCID • U Like • D Dislike • I/Esc Close"
	if !strings.Contains(view, footer) {
		t.Fatalf("compact TELA info footer missing: %q", view)
	}
	for _, hidden := range []string{"S Sort", "O Ord", "F Filter"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("info footer should hide %q: %q", hidden, view)
		}
	}
}

func TestDiscoverLikeRequiresConfirmation(t *testing.T) {
	m := NewDiscover()
	m.SetTela([]wallet.CatalogEntry{{
		SCID: strings.Repeat("a", 64), Class: "TELA-INDEX-1", Name: "Likeable app",
	}}, false, false)

	m, _ = m.Update(tea.KeyPressMsg{Text: "u"})
	if m.votePending == false || m.voting || m.wantVote {
		t.Fatalf("like should wait for confirmation: pending=%v voting=%v wantVote=%v", m.votePending, m.voting, m.wantVote)
	}
	if !containsStr(m.View(), "Submit like") || !containsStr(m.View(), "0.001 DERO") {
		t.Fatalf("confirmation prompt missing: %q", m.View())
	}

	m, _ = m.Update(tea.KeyPressMsg{Text: "n"})
	if m.votePending || m.voteSCID != "" {
		t.Fatal("N should cancel the pending vote")
	}
}

func TestDiscoverDislikeConfirmProducesVote(t *testing.T) {
	scid := strings.Repeat("b", 64)
	m := NewDiscover()
	m.SetTela([]wallet.CatalogEntry{{SCID: scid, Class: "TELA-INDEX-1", Name: "Dislikeable app"}}, false, false)

	m, _ = m.Update(tea.KeyPressMsg{Text: "d"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "y"})
	gotSCID, like, ok := m.WantVote()
	if !ok || gotSCID != scid || like {
		t.Fatalf("WantVote = (%q, %v, %v), want (%q, false, true)", gotSCID, like, ok, scid)
	}
	if !m.voting || m.votePending {
		t.Fatalf("dislike should enter submitting state: voting=%v pending=%v", m.voting, m.votePending)
	}
}

func TestDiscoverVoteOnlyWorksOnTelaTab(t *testing.T) {
	m := NewDiscover()
	m.SetCatalog(nil, []wallet.CatalogEntry{{SCID: strings.Repeat("c", 64), Name: "NFT"}}, nil, false)
	m, _ = m.Update(tea.KeyPressMsg{Text: "2"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "u"})
	if m.votePending {
		t.Fatal("NFTs must not expose TELA voting")
	}
}

func TestDiscoverVoteMarkedAfterSubmission(t *testing.T) {
	scid := strings.Repeat("d", 64)
	m := NewDiscover()
	m.SetTela([]wallet.CatalogEntry{{SCID: scid, Class: "TELA-INDEX-1", Name: "App"}}, false, false)
	m.MarkVoted(scid)
	if !m.HasVoted(scid) {
		t.Fatal("MarkVoted should block repeat submissions")
	}
	m, _ = m.Update(tea.KeyPressMsg{Text: "u"})
	if m.votePending {
		t.Fatal("already-voted app should not open a new vote prompt")
	}
}
