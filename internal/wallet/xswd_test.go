// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"strings"
	"testing"

	"github.com/civilware/epoch"
	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgnative "github.com/hypergnomon/hypergnomon/storage"
	"github.com/hypergnomon/hypergnomon/structures"
)

// TestEpochHandlerMethods verifies the EPOCH library exposes exactly the
// methods we register over XSWD and that the registration set matches.
func TestEpochHandlerMethods(t *testing.T) {
	handler := epoch.GetHandler()
	for name := range epochMethods {
		if name == "AttemptEPOCHWithAddr" {
			continue // Engram method, registered by startEpoch
		}
		if _, ok := handler[name]; !ok {
			t.Errorf("epoch.GetHandler() missing method %q", name)
		}
	}
}

// TestIsEpochMethod verifies auto-allowed EPOCH methods are recognized and
// sensitive wallet methods are never auto-allowed.
func TestIsEpochMethod(t *testing.T) {
	allowed := []string{
		"AttemptEPOCH",
		"AttemptEPOCHWithAddr",
		"SubmitEPOCH",
		"GetMaxHashesEPOCH",
		"GetAddressEPOCH",
		"GetSessionEPOCH",
		"StopEPOCH",
	}
	for _, m := range allowed {
		if !isEpochMethod(m) {
			t.Errorf("isEpochMethod(%q) = false, want true", m)
		}
	}

	denied := []string{
		"transfer",
		"query_key",
		"QueryKey",
		"GetBalance",
		"SignData",
		"Subscribe",
		"",
	}
	for _, m := range denied {
		if isEpochMethod(m) {
			t.Errorf("isEpochMethod(%q) = true, want false", m)
		}
	}
}

func TestEpochGetWorkPort(t *testing.T) {
	cases := []struct {
		network, bind string
		want          int
	}{
		{"Mainnet", "", 10100},
		{"Testnet", "", 40400},
		{"Simulator", "", 20003},
		{"simulator", "", 20003},
		{"Mainnet", "0.0.0.0:10100", 10100},
		{"Simulator", "0.0.0.0:20003", 20003},
		{"Mainnet", "127.0.0.1:10900", 10900},
		{"Mainnet", "bad", 10100},
	}
	for _, tc := range cases {
		if got := epochGetWorkPort(tc.network, tc.bind); got != tc.want {
			t.Errorf("epochGetWorkPort(%q, %q) = %d, want %d", tc.network, tc.bind, got, tc.want)
		}
	}
}

func TestAttemptEPOCHWithAddrEmpty(t *testing.T) {
	_, err := attemptEPOCHWithAddr(context.Background(), attemptWithAddrParams{Hashes: 1}, nil, "127.0.0.1:10102")
	if err == nil {
		t.Fatal("empty address should fail")
	}
}

func TestIsGnomonMethod(t *testing.T) {
	if !isGnomonMethod("Gnomon.GetAllSCIDVariableDetails") {
		t.Fatal("Gnomon.* should auto-allow")
	}
	if isGnomonMethod("GetAllSCIDVariableDetails") {
		t.Fatal("bare name is not a Gnomon method")
	}
}

func TestIsDeroMethod(t *testing.T) {
	if !isDeroMethod("DERO.GetSC") || !isDeroMethod("GetSC") {
		t.Fatal("DERO.GetSC / GetSC should auto-allow")
	}
	if isDeroMethod("GetAddress") {
		t.Fatal("wallet methods must still prompt")
	}
}

func TestGnomonGetAllSCIDVariableDetailsInactive(t *testing.T) {
	_, err := gnomonGetAllSCIDVariableDetails(context.Background(), gnomonSCIDParam{SCID: "aa"})
	if err == nil || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("got %v", err)
	}
}

func TestGnomonGetAllSCIDVariableDetailsLatest(t *testing.T) {
	store, err := hgstorage.NewBBoltDB(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	scid := strings.Repeat("a", 64)
	batch := hgnative.NewWriteBatch()
	batch.AddVariables(scid, 100, []*structures.SCIDVariable{{Key: "name", Value: "derobeats"}})
	batch.AddInteractionHeight(scid, 100)
	batch.LastHeight = 100
	if err := store.Inner().FlushBatch(batch); err != nil {
		t.Fatal(err)
	}
	if got := store.GetAllSCIDVariableDetails(scid); len(got) != 0 {
		t.Fatalf("GetAllSCIDVariableDetails(height 0) should be empty, got %d", len(got))
	}

	globalAppHyperMu.Lock()
	prev := globalAppHyper
	globalAppHyper = &HyperGnomon{store: store}
	globalAppHyperMu.Unlock()
	t.Cleanup(func() {
		globalAppHyperMu.Lock()
		globalAppHyper = prev
		globalAppHyperMu.Unlock()
	})

	got, err := gnomonGetAllSCIDVariableDetails(context.Background(), gnomonSCIDParam{SCID: strings.ToUpper(scid)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AllVariables) != 1 {
		t.Fatalf("latest vars: %+v", got.AllVariables)
	}
}

func TestLiveSCVariablesEmpty(t *testing.T) {
	if liveSCVariables("", "aa") != nil {
		t.Fatal("empty endpoint")
	}
	if liveSCVariables("127.0.0.1:10102", "") != nil {
		t.Fatal("empty scid")
	}
}
