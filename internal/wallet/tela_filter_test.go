// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"reflect"
	"testing"
)

func TestFilterLaunchableTela(t *testing.T) {
	app := CatalogEntry{SCID: "aa", DURL: "vault.tela", InstallHeight: 100}
	newer := CatalogEntry{SCID: "bb", DURL: "vault.tela", InstallHeight: 500}
	rated := CatalogEntry{SCID: "cc", DURL: "vault.tela", InstallHeight: 500, AvgRating: 8}
	other := CatalogEntry{SCID: "dd", DURL: "chat.tela", InstallHeight: 200}

	cases := []struct {
		name string
		in   []CatalogEntry
		want []string // SCIDs
	}{
		{"nil", nil, nil},
		{"empty dURL", []CatalogEntry{{SCID: "aa"}}, nil},
		{"lib", []CatalogEntry{{SCID: "aa", DURL: "std.lib"}}, nil},
		{"library", []CatalogEntry{{SCID: "aa", DURL: "std.library"}}, nil},
		{"shards", []CatalogEntry{{SCID: "aa", DURL: "pack.shards"}}, nil},
		{"shard", []CatalogEntry{{SCID: "aa", DURL: "pack.shard"}}, nil},
		{"bootstrap", []CatalogEntry{{SCID: "aa", DURL: "list.bootstrap"}}, nil},
		{"case", []CatalogEntry{{SCID: "aa", DURL: "Std.LIB"}}, nil},
		{"keep app", []CatalogEntry{app}, []string{"aa"}},
		{"collapse newest", []CatalogEntry{app, newer}, []string{"bb"}},
		{"same height prefers rating", []CatalogEntry{newer, rated}, []string{"cc"}},
		{"scid dedup", []CatalogEntry{app, {SCID: "AA", DURL: "vault.tela", InstallHeight: 999}}, []string{"aa"}},
		{"two apps", []CatalogEntry{app, other}, []string{"aa", "dd"}},
		{"mixed", []CatalogEntry{
			{SCID: "x", DURL: "junk.lib"},
			app, newer,
			{SCID: "y", DURL: "list.bootstrap"},
			other,
		}, []string{"bb", "dd"}},
		{"logo name", []CatalogEntry{{SCID: "aa", Name: "dApps with Logo", DURL: "dapps.tela"}}, nil},
		{"logo durl", []CatalogEntry{{SCID: "aa", DURL: "brand.logo"}}, nil},
		{"index.html", []CatalogEntry{{SCID: "aa", DURL: "index.html"}}, nil},
		{"png", []CatalogEntry{{SCID: "aa", Name: "icon.png", DURL: "icon.tela"}}, nil},
		{"bare index", []CatalogEntry{{SCID: "aa", DURL: "index"}}, nil},
		{"hammer dupes", []CatalogEntry{
			{SCID: "aa", Name: "Crypto Hammer", DURL: "crypto-hammer.tela", InstallHeight: 100},
			{SCID: "bb", Name: "CryptoHammer", DURL: "ch.tela", InstallHeight: 200},
		}, []string{"bb"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterLaunchableTela(tc.in)
			var scids []string
			for _, e := range got {
				scids = append(scids, e.SCID)
			}
			if tc.want == nil {
				if len(scids) != 0 {
					t.Fatalf("got %v, want empty", scids)
				}
				return
			}
			if !reflect.DeepEqual(scids, tc.want) {
				t.Fatalf("got %v, want %v", scids, tc.want)
			}
		})
	}
}
