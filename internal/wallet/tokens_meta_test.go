// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import "testing"

func TestApplyTokenMetadataQuotedReturn(t *testing.T) {
	code := `
Function Name() String
    10 RETURN "DERO Ducks"
End Function
Function Symbol() String
    10 RETURN "DUCK"
End Function
Function Decimals() Uint64
    10 RETURN 0
End Function
`
	info := TokenInfo{}
	applyTokenMetadata(&info, code, nil)
	if info.Name != "DERO Ducks" {
		t.Fatalf("name=%q", info.Name)
	}
	if info.Ticker != "DUCK" {
		t.Fatalf("ticker=%q", info.Ticker)
	}
	if info.Decimals != 0 {
		t.Fatalf("decimals=%d", info.Decimals)
	}
}

func TestApplyTokenMetadataLoadKey(t *testing.T) {
	code := `
Function Name() String
    10 RETURN LOAD("n")
End Function
Function Symbol() String
    10 RETURN LOAD("s")
End Function
`
	vals := map[string]string{"n": "Cake Token", "s": "CAKE"}
	info := TokenInfo{}
	applyTokenMetadata(&info, code, vals)
	if info.Name != "Cake Token" {
		t.Fatalf("name=%q want Cake Token (must not use LOAD key as name)", info.Name)
	}
	if info.Ticker != "CAKE" {
		t.Fatalf("ticker=%q", info.Ticker)
	}
}

func TestApplyTokenMetadataStoreKeys(t *testing.T) {
	info := TokenInfo{}
	applyTokenMetadata(&info, "", map[string]string{
		"name":     "My Token",
		"symbol":   "MTK",
		"decimals": "5",
	})
	if info.Name != "My Token" || info.Ticker != "MTK" || info.Decimals != 5 {
		t.Fatalf("got %+v", info)
	}
}

func TestApplyTokenMetadataJSON(t *testing.T) {
	info := TokenInfo{}
	applyTokenMetadata(&info, "", map[string]string{
		"metadata": `{"name":"Duck #1","symbol":"DUCK"}`,
	})
	if info.Name != "Duck #1" || info.Ticker != "DUCK" {
		t.Fatalf("got %+v", info)
	}
}

func TestDecodeSCStringHex(t *testing.T) {
	if got := decodeSCString("4475636b"); got != "Duck" {
		t.Fatalf("got %q", got)
	}
	if got := decodeSCString("NOT AVAILABLE err: x"); got != "" {
		t.Fatalf("got %q", got)
	}
}
