// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"path/filepath"
	"strings"
)

var telaArtifactSuffixes = []string{".lib", ".library", ".shards", ".shard", ".bootstrap"}

// FilterLaunchableTela drops INDEX artifacts and collapses duplicate apps to
// the newest install (then best rating, then SCID).
func FilterLaunchableTela(in []CatalogEntry) []CatalogEntry {
	if len(in) == 0 {
		return nil
	}
	best := make(map[string]CatalogEntry, len(in))
	order := make([]string, 0, len(in))
	seenSCID := make(map[string]bool, len(in))
	for _, e := range in {
		if telaJunk(e) {
			continue
		}
		scid := strings.ToLower(strings.TrimSpace(e.SCID))
		if scid == "" || seenSCID[scid] {
			continue
		}
		seenSCID[scid] = true
		key := telaGroupKey(e)
		if key == "" {
			continue
		}
		if prev, ok := best[key]; ok {
			if !telaPrefer(e, prev) {
				continue
			}
		} else {
			order = append(order, key)
		}
		best[key] = e
	}
	out := make([]CatalogEntry, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

func telaJunk(e CatalogEntry) bool {
	durl := strings.ToLower(strings.TrimSpace(e.DURL))
	name := strings.ToLower(strings.TrimSpace(e.Name))
	if durl == "" || telaArtifactDURL(durl) {
		return true
	}
	if strings.Contains(name, "logo") || strings.Contains(durl, "logo") {
		return true
	}
	return telaFileLike(name) || telaFileLike(durl)
}

func telaArtifactDURL(durl string) bool {
	for _, s := range telaArtifactSuffixes {
		if strings.HasSuffix(durl, s) {
			return true
		}
	}
	return false
}

func telaFileLike(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	ext := strings.TrimPrefix(filepath.Ext(s), ".")
	switch ext {
	case "html", "htm", "css", "js", "png", "jpg", "jpeg", "gif", "svg", "ico", "webp", "json", "txt", "md":
		return true
	}
	if ext != "" {
		return false
	}
	switch filepath.Base(s) {
	case "index", "favicon", "style", "main":
		return true
	}
	return false
}

func telaGroupKey(e CatalogEntry) string {
	if nk := telaCanon(e.Name); len(nk) >= 4 {
		return nk
	}
	return telaCanon(e.DURL)
}

func telaCanon(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".tela")
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b = append(b, c)
		}
	}
	return string(b)
}

func telaPrefer(a, b CatalogEntry) bool {
	if a.InstallHeight != b.InstallHeight {
		return a.InstallHeight > b.InstallHeight
	}
	if a.AvgRating != b.AvgRating {
		return a.AvgRating > b.AvgRating
	}
	return a.SCID < b.SCID
}
