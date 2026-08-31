// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"reflect"
	"testing"
)

func TestBrowserCmd(t *testing.T) {
	link := "http://127.0.0.1:8082/index.html"
	cases := []struct {
		goos string
		name string
		args []string
	}{
		{"linux", "xdg-open", []string{link}},
		{"freebsd", "xdg-open", []string{link}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", link}},
		{"darwin", "open", []string{link}},
		{"plan9", "", nil},
	}
	for _, tc := range cases {
		name, args := browserCmd(tc.goos, link)
		if name != tc.name || !reflect.DeepEqual(args, tc.args) {
			t.Fatalf("%s: got %s %v, want %s %v", tc.goos, name, args, tc.name, tc.args)
		}
	}
}

func TestServeTelaInvalidSCID(t *testing.T) {
	if _, err := ServeTela("aa", "127.0.0.1:10102", nil); err == nil {
		t.Fatal("short SCID should fail")
	}
}
