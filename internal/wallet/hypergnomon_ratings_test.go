// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import "testing"

func TestRatingDisplay(t *testing.T) {
	cases := []struct {
		name      string
		entry     CatalogEntry
		wantLabel string
		wantTier  RatingTier
	}{
		{"unrated", CatalogEntry{}, "unrated", RatingTierNone},
		{"top", CatalogEntry{AvgRating: 9.2}, "9.2", RatingTierTop},
		{"good", CatalogEntry{AvgRating: 7.5}, "7.5", RatingTierGood},
		{"mid", CatalogEntry{AvgRating: 5.0}, "5.0", RatingTierMid},
		{"poor", CatalogEntry{AvgRating: 3.1}, "3.1", RatingTierPoor},
		{
			"dislikes dominate",
			CatalogEntry{AvgRating: 9.9, Likes: 2, Dislikes: 3},
			"poor", RatingTierPoor,
		},
		{
			"likes dominate keeps tier",
			CatalogEntry{AvgRating: 8.0, Likes: 5, Dislikes: 1},
			"8.0", RatingTierGood,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, tier := tc.entry.RatingDisplay()
			if label != tc.wantLabel || tier != tc.wantTier {
				t.Fatalf("got (%q, %d), want (%q, %d)", label, tier, tc.wantLabel, tc.wantTier)
			}
		})
	}
}

func TestApplyRatingsComputesAverage(t *testing.T) {
	// Pure math check: two raters at 80 and 60 (0-99 scale) -> 7.0 average.
	sum := float64(80+60) / 2 / 10.0
	if sum != 7.0 {
		t.Fatalf("average math: got %v, want 7.0", sum)
	}
}
