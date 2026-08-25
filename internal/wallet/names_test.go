// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/rpc"
)

// newTestAddress generates a random mainnet address and returns its string form
// plus the hex-encoded compressed public key (the form stored in the NameService
// SC data tree).
func newTestAddress(t *testing.T, mainnet bool) (addrStr, hexKey string) {
	t.Helper()

	pub := crypto.GPoint.ScalarMult(crypto.RandomScalarBNRed())
	addr := rpc.NewAddressFromKeys(pub)
	addr.Mainnet = mainnet

	return addr.String(), hex.EncodeToString(pub.EncodeCompressed())
}

func TestNameServiceSCID(t *testing.T) {
	for i := 0; i < 31; i++ {
		if NameServiceSCID[i] != 0 {
			t.Fatalf("NameServiceSCID byte %d = %d, want 0", i, NameServiceSCID[i])
		}
	}
	if NameServiceSCID[31] != 1 {
		t.Fatalf("NameServiceSCID byte 31 = %d, want 1", NameServiceSCID[31])
	}
}

func TestValidateNameForRegistration(t *testing.T) {
	tests := []struct {
		name    string
		wantErr string
	}{
		{name: "alice", wantErr: ""},
		{name: "  alice  ", wantErr: ""}, // trimmed before validation
		{name: "a.b-c_d", wantErr: ""},
		{name: "", wantErr: "empty"},
		{name: "   ", wantErr: "empty"},
		{name: strings.Repeat("a", 64), wantErr: "less than 64"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.wantErr, func(t *testing.T) {
			err := validateNameForRegistration(tt.name)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestRegisterNameRequiresOpenWallet(t *testing.T) {
	w := &Wallet{}
	_, err := w.RegisterName(context.Background(), "alice")
	if err == nil || !strings.Contains(err.Error(), "wallet not open") {
		t.Fatalf("expected 'wallet not open' error, got: %v", err)
	}
}

func TestTransferNameRequiresOpenWallet(t *testing.T) {
	w := &Wallet{}
	_, err := w.TransferName(context.Background(), "alice", "dero1recipient")
	if err == nil || !strings.Contains(err.Error(), "wallet not open") {
		t.Fatalf("expected 'wallet not open' error, got: %v", err)
	}
}

func TestListRegisteredNamesRequiresOpenWallet(t *testing.T) {
	w := &Wallet{}
	_, err := w.ListRegisteredNames(context.Background(), "localhost:10102")
	if err == nil || !strings.Contains(err.Error(), "wallet not open") {
		t.Fatalf("expected 'wallet not open' error, got: %v", err)
	}
}

func TestFilterOwnedNames(t *testing.T) {
	// Two distinct mainnet addresses: one owned, one foreign.
	ownedAddr, ownedKey := newTestAddress(t, true)
	_, foreignKey := newTestAddress(t, true)

	t.Run("filters owned names", func(t *testing.T) {
		varKeys := map[string]interface{}{
			"alice": ownedKey,
			"bob":   foreignKey,
		}

		entries := filterOwnedNames(varKeys, ownedAddr, false)
		if len(entries) != 1 {
			t.Fatalf("expected 1 owned name, got %d", len(entries))
		}
		if entries[0].Name != "alice" {
			t.Fatalf("expected name 'alice', got %q", entries[0].Name)
		}
		if entries[0].Owner != ownedAddr {
			t.Fatalf("expected owner %q, got %q", ownedAddr, entries[0].Owner)
		}
	})

	t.Run("skips reserved metadata keys", func(t *testing.T) {
		varKeys := map[string]interface{}{
			"owner": ownedKey,
			"own1":  ownedKey,
			"C":     ownedKey,
			"alice": ownedKey,
		}

		entries := filterOwnedNames(varKeys, ownedAddr, false)
		if len(entries) != 1 {
			t.Fatalf("expected 1 owned name after skipping reserved keys, got %d", len(entries))
		}
		if entries[0].Name != "alice" {
			t.Fatalf("expected name 'alice', got %q", entries[0].Name)
		}
	})

	t.Run("skips malformed values", func(t *testing.T) {
		varKeys := map[string]interface{}{
			"short":     "00ff",          // not 33 bytes
			"nothex":    "zznothexvalue", // invalid hex
			"notstring": 12345,           // wrong type
			"alice":     ownedKey,
		}

		entries := filterOwnedNames(varKeys, ownedAddr, false)
		if len(entries) != 1 {
			t.Fatalf("expected 1 owned name after skipping malformed values, got %d", len(entries))
		}
		if entries[0].Name != "alice" {
			t.Fatalf("expected name 'alice', got %q", entries[0].Name)
		}
	})

	t.Run("respects testnet network flag", func(t *testing.T) {
		// Build a testnet address (deto1...) and confirm the network flag is
		// applied when resolving stored compressed keys.
		testAddr, testKey := newTestAddress(t, false)

		varKeys := map[string]interface{}{
			"carol": testKey,
		}

		entries := filterOwnedNames(varKeys, testAddr, true)
		if len(entries) != 1 {
			t.Fatalf("expected 1 owned testnet name, got %d", len(entries))
		}
		if entries[0].Name != "carol" {
			t.Fatalf("expected name 'carol', got %q", entries[0].Name)
		}
		if entries[0].Owner != testAddr {
			t.Fatalf("expected owner %q, got %q", testAddr, entries[0].Owner)
		}
	})

	t.Run("returns empty for empty map", func(t *testing.T) {
		entries := filterOwnedNames(map[string]interface{}{}, ownedAddr, false)
		if len(entries) != 0 {
			t.Fatalf("expected 0 entries, got %d", len(entries))
		}
	})
}
