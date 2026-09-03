// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"fmt"
	"strings"

	"github.com/civilware/tela"
)

const telaVoteFee = "0.001 DERO"

// TelaVoteValue maps the TUI's binary vote to TELA's 0-99 rating protocol.
// TELA treats values above 49 as positive and values below 50 as negative.
func TelaVoteValue(like bool) uint64 {
	if like {
		return 99
	}
	return 0
}

// RateTELA submits one positive or negative vote for a TELA app. The TELA
// library signs the canonical SC call and charges its fixed 0.001 DERO fee.
func (w *Wallet) RateTELA(ctx context.Context, scid string, like bool) TransferResult {
	if w == nil || w.wallet == nil {
		return TransferResult{Error: "wallet not open"}
	}
	if err := ValidateSCID(strings.TrimSpace(scid)); err != nil {
		return TransferResult{Error: err.Error()}
	}
	if err := ctx.Err(); err != nil {
		return TransferResult{Error: err.Error()}
	}

	txID, err := tela.Rate(w.wallet, strings.TrimSpace(scid), TelaVoteValue(like))
	if err != nil {
		return TransferResult{Error: fmt.Sprintf("TELA vote failed: %v", err)}
	}
	return TransferResult{TxID: txID, Status: "success"}
}
