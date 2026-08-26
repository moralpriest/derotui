# Design.md – DERO TUI Design System

> Source of truth for visual consistency. Extracted from `internal/ui/styles/styles.go`, `AGENTS.md`, and page Views.

## 1. Theme

Default: `neon` (see `styles.go:ApplyTheme`).

| Token | Value | Usage |
|-------|-------|-------|
| `ColorPrimary` | `#BF40FF` neon purple | Title, focused border, selection `▸` |
| `ColorBg` | `#000000` | Background |
| `ColorCardBg` | `#1E1B2E` | Card/box background |
| `ColorSuccess` | `#10B981` green | Synced, Mining, running border |
| `ColorWarning` | `#F59E0B` orange | Syncing, Stopped |
| `ColorError` | `#EF4444` | Errors / Rejected >0 |
| `ColorAccent` | `#06B6D4` cyan | Threads, accent |
| `ColorSecondary` | muted purple | Secondary highlights |
| `ColorBorder` | muted gray | Box border (inactive) |
| `ColorText` | white | Primary text |
| `ColorMuted` | gray | Labels, footer hints |
| `Width` | `80` | Outer box width (`ThemedBoxStyle`) |
| `InputWidth` | `60` | `components.Input` width (`ti.SetWidth(56)`) |

Styles: `TitleStyle` (Primary Bold), `MutedStyle`, `TextStyle`, `Success/Warning/Error/AccentStyle`, `BoxStyle` (RoundedBorder), `ThemedBoxStyle`, `InputStyle/Focused/Disabled`, `HeaderStyle`.

## 2. Layout

* **Centering:** All pages center via `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)` – `width`/`height` from `tea.WindowSizeMsg` (`m.width/m.height`), fallback `80×24`. Miner fixed to centered (was left-aligned).
* **Box:** `ThemedBoxStyle().Width(Width).Padding(2,4)` – inner content `72` wide. When mining, `BorderForeground(ColorSuccess)` else `ColorBorder`.
* **Header/Footer:** No global top bar. Daemon shown once inside page (`Mining via …` footer in miner). Footer: `AccentStyle(k) + MutedStyle(label)` joined by `•`, single line, responsive truncation.

## 3. Typography & Rows

Rows built as `[]string` then `strings.Join(rows, "\n")` inside box.
* Label: `MutedStyle.Render("Label: ")`
* Value: per **Option C (status-driven)** – see §6.
* Truncation: `truncatePlain(addr, 56)` + `…` for long hashes/addresses.

## 4. Components

* **Input:** `components.NewInput(label, placeholder, false)` – `label` rendered above input (`label+"\n"+input`). `InputStyle` vs `FocusedInputStyle` (Primary border). Do **not** do `Muted("Label: ")+input.View()` inline – breaks border. Miner uses `NewInput("Address", "dero1…", false)` + `rows=append(rows, input.View())`.
* **Keys:** `key.NewBinding(key.WithKeys("s"))` etc. Footer shows `S Start • ←/→ Threads • A Edit Address • C Copy • / Commands • Esc Back` – responsive: drop `C Copy` then abbreviate `Edit Address→Edit`, `Commands→Cmd` when `lipgloss.Width(joined) > availWidth`.

## 5. Page Pattern

```go
type Model struct { page Page; width, height int; wallet ...; miner pages.MinerModel ... }
func (m Model) View() tea.View {
  var content string
  switch m.page { case PageMiner: content = m.miner.View() ... }
  // palette overlay, debug, then return tea.NewView(content)
}
```

Each `pages/*.go` follows `New / Init / Update(msg tea.Msg) / View() string`; `Update` handles `tea.WindowSizeMsg` and `tea.KeyPressMsg`; `View` returns centered box.

## 6. Miner Page Specifics (Option C – Status-Driven)

* **Default threads:** `maxThreads-2` (min 1) – `internal/ui/pages/miner.go:NewMiner()`.
* **Visual (single):** Spinner only on `Status:` line via `renderStatus()` → `SuccessStyle(spinner+" Mining")` where `spinner = [⠋…⠏][frame%10]` tick 180ms (`SpinnerTickMsg`). Title static `Miner`. Green border when `Running`.
* **Value colors (C):**
  * `Status: Stopped` → `Warning` orange; `Status: ∴ Mining` → `Success` green
  * `Threads: 30/32` → `Accent` cyan (always)
  * `Hashrate:` → `Success` if `>0` else `Muted`
  * `Minis`/`Blocks:` → `Success` if `>0` else `Muted`
  * `Rejected:` → `Error` red if `>0` else `Muted`
  * `Total Hashes, Height, Difficulty, Uptime` → `Text` white (or `Muted` when 0)
* **Address:** `Address` label via `components.Input` (label "Address"), `View` shows `input.View()` when `editingAddress` else `renderAddress()` truncated. `a`/`e` edit, `enter` save (validate `rpc.NewAddress`, `config.SetLastMiningAddress`), `esc` cancel, `c` copy. Threads `←/→` disabled when `Running`.
* **Daemon:** Shown once as footer `Mining via <host>` (from `m.miner.DaemonHost` fallback `Opts.DaemonAddress`), not in top bar.
* **Footer:** Single line, `availWidth = m.width-8` (fallback 80), drop `C Copy` first, then abbreviate, `lipgloss.Width` check.

## 7. Verification

* `go vet ./...` / `go test ./...`
* Manual: `/miner` centered like `/send`/`/daemon`, `Status: ∴ Mining` spinner only, values colored per C, footer 1 line at 80 and 60 cols, address textbox clean.
