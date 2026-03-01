// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import "strings"

func maxLabelWidth(labels []string) int {
	max := 0
	for _, label := range labels {
		if len(label) > max {
			max = len(label)
		}
	}
	return max
}

func padLabel(label string, width int) string {
	if len(label) >= width {
		return label
	}
	return label + strings.Repeat(" ", width-len(label))
}
