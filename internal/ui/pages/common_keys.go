// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import "charm.land/bubbles/v2/key"

var (
	pageEscKeys       = key.NewBinding(key.WithKeys("esc", "escape"))
	pageEnterKeys     = key.NewBinding(key.WithKeys("enter"))
	pageTabKeys       = key.NewBinding(key.WithKeys("tab"))
	pageShiftTabKeys  = key.NewBinding(key.WithKeys("shift+tab", "backtab"))
	pageCopyKeys      = key.NewBinding(key.WithKeys("c", "C"))
	pageNextFieldKeys = key.NewBinding(key.WithKeys("tab", "down"))
	pagePrevFieldKeys = key.NewBinding(key.WithKeys("shift+tab", "up"))
	pageUpKeys        = key.NewBinding(key.WithKeys("up"))
	pageDownKeys      = key.NewBinding(key.WithKeys("down"))
)
