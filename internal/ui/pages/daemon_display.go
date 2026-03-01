// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import "strings"

func stripDaemonScheme(addr string) string {
	s := strings.TrimSpace(addr)
	for _, prefix := range []string{"https://", "http://", "wss://", "ws://"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			return s[len(prefix):]
		}
	}
	return s
}
