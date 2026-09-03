// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/wallet"
)

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
