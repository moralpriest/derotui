// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"strings"
	"testing"
)

func TestTelaVoteValue(t *testing.T) {
	if got := TelaVoteValue(true); got != 99 {
		t.Fatalf("like vote value = %d, want 99", got)
	}
	if got := TelaVoteValue(false); got != 0 {
		t.Fatalf("dislike vote value = %d, want 0", got)
	}
}

func TestRateTELARequiresOpenWallet(t *testing.T) {
	result := (&Wallet{}).RateTELA(context.Background(), strings.Repeat("a", 64), true)
	if result.Error != "wallet not open" {
		t.Fatalf("error = %q, want wallet not open", result.Error)
	}
}

func TestRateTELAValidatesSCID(t *testing.T) {
	result := (&Wallet{}).RateTELA(context.Background(), "bad", true)
	if result.Error != "wallet not open" {
		t.Fatalf("closed wallet should be checked first, got %q", result.Error)
	}
}
