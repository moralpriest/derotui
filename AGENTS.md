# AGENTS.md - DERO Wallet TUI Development Guide

This repository contains a modern TUI wallet for the DERO blockchain built with Bubble Tea.

## Build Commands

```bash
# Build the TUI wallet binary
go build -o derotui ./cmd/

# Run all tests
go test -v ./...

# Format code
go fmt ./...

# Static analysis
go vet ./...
```

## Project Structure

```
cmd/
└── main.go                          # Entry point, CLI parsing, daemon connectivity setup

internal/
├── config/
│   └── config.go                    # User config (~/.derotui.json) for last wallet

├── ui/
│   ├── app.go                       # Main application model, page routing, message handling
│   ├── mouse.go                     # MouseablePage interface definition
│   ├── components/
│   │   └── input.go                 # Styled text input wrapper
│   ├── pages/
│   │   ├── dashboard.go             # Main dashboard with balance, activity, quick actions
│   │   ├── daemon.go                # Daemon connection input page
│   │   ├── history.go               # Transaction history (custom color-coded table)
│   │   ├── integratedaddr.go        # Integrated address (payment request) generation
│   │   ├── keyinput.go              # Hex key input/display (restore & view)
│   │   ├── network.go               # Network selection (Mainnet/Testnet/Simulator)
│   │   ├── password.go              # Password input (unlock/create)
│   │   ├── qrcode.go                # QR code display page
│   │   ├── seed.go                  # Seed phrase input/display (restore & view)
│   │   ├── send.go                  # Send DERO form with Payment ID support
│   │   ├── txdetails.go             # Transaction details with copy keybindings
│   │   ├── welcome.go               # Welcome page with slash command menu
│   │   └── xswd.go                  # XSWD authorization and permission dialogs
│   └── styles/
│       └── styles.go                # Lipgloss styles, theme colors, ASCII art
└── wallet/
    ├── wallet.go                    # Wallet wrapper around DERO walletapi
    └── types.go                     # Type definitions (WalletInfo, TransactionInfo, etc.)
```

## Features Implemented

### Theme
- **Primary Color**: Neon purple (#BF40FF)
- **Background**: Black (#000000)
- **Card Background**: Dark purple (#1E1B2E)
- **Accent colors**: 
  - Green (#10B981) for success/synced
  - Orange (#F59E0B) for warning/syncing
  - Cyan (#06B6D4) for simulator indicator
  - Red (#EF4444) for errors

### Welcome Page
- **Slash command menu**: Type "/" to open command autocomplete
  - `/open` - Open an existing wallet (file picker)
  - `/create` - Create a new wallet
  - `/restore` - Restore wallet (submenu: From Seed, From Key)
  - `/connect` - Connect to a daemon
  - `/exit` - Exit the application
- **Menu opens upward** above the input field
- **Daemon status display**: Shows "Block: <height>" and "Daemon: <address>"
  - Block number is green when synced, yellow when syncing
  - Daemon address shown in green when connected

### Auto-Open Last Wallet
- Stores last opened wallet path in `~/.derotui.json`
- On startup, automatically prompts for password if a previous wallet exists
- Shows wallet file path in the password prompt
- Supports multiple wallet networks (Mainnet, Testnet, Simulator) saved per-wallet

### Dashboard
- **Balance display**: Large balance with DERO symbol (◆)
- **Recent Activity**: Shows last 5 transactions
- **Paginated Quick Actions** (2 pages):
  - **Page 1 - Core:**
    - `[S]` Send DERO
    - `[C]` Copy Address
    - `[Y]` QR Code
    - `[H]` History
    - `[G]` Register Wallet (shown only when unregistered)
  - **Page 2 - Advanced:**
    - `[V]` View Seed
    - `[K]` View Hex Key
    - `[P]` Change Password
    - `[R]` Payment Request (Integrated Address)
    - `[X]` XSWD Server Toggle
- **Address display** at bottom (truncated to 50 chars)
- **Footer**: Shows all keyboard shortcuts with page indicator
- **Flash messages**: Success/error notifications (green/red)

### Seed Phrase Display/Input
- **Display mode**: Electrum-style card grid with 25 cards (5x5)
  - Each card shows: word number (gray) + word (neon purple, bold)
  - Card background: dark purple (#1E1B2E)
  - Gap between cards: 1 character
- **Input mode**: Textarea for entering 25-word seed
- **Real-time validation**: Words validated against BIP39 wordlist as you type
- **Error persistence**: Errors stay visible until user starts typing
- **Copy to clipboard**: Press `C` to copy seed phrase

### Hex Key Display/Input
- **Display mode**: Shows 64-character hex key split into two lines
- **Input mode**: Text input for 64-character hex key
- **Validation**: Checks length and hex characters
- **Copy to clipboard**: Press `C` to copy hex key

### QR Code Display
- Displays wallet receive address as QR code
- Uses Unicode half-block characters for compact rendering
- Shows full address in text below QR code
- **Copy**: Press `C` to copy address

### Daemon Connection
- **Auto-detect local daemon** on startup (Mainnet, Testnet, Simulator)
- **Multi-daemon detection**: Prompts user to select if multiple daemons detected
- **Keep_Connectivity()** background goroutine maintains connection
- **Periodic status refresh** every 5 seconds
- **Default ports**: Mainnet 10102, Testnet 40402, Simulator 20000
- **Placeholder shows** appropriate localhost:port for guidance
- **Auto-append port** if user doesn't specify one

### Transaction History
- **Custom table implementation** (not using bubbles table)
- **Color coding**: 
  - Green text for MINED/IN transactions
  - Red text for OUT transactions
- **Full-row selection**: Purple background + yellow text across all columns
- **Manual scroll**: Cursor + offset management
- **Columns**: Date, Type, Amount, Fee, Confirmations
- **Mouse support**: Click to select, double-click for details
- **Keyboard**: Arrow keys to navigate, Enter for details

### Transaction Details
- **Section-based layout** with headers (`── Section Name ─────────`)
- **Sections**: Overview, Block, Transaction, Payment ID (conditional)
- **Fields**: Type, Status, Amount, Fee, Burn, Height, Topo Height, Timestamp, TxID, Block Hash, Sender, Destination, Proof, Message, Payment ID/Port
- **Word wrapping**: Long messages are wrapped to fit width (52 chars visible per line)
- **Truncation**: Hashes/addresses truncated at 52 chars with `...`
- **Copy keybindings**:
  - `[T]` Copy TxID
  - `[B]` Copy Block Hash
  - `[H]` Copy Height
  - `[S]` Copy Sender
  - `[D]` Copy Destination
  - `[P]` Copy Proof
  - `[M]` Copy Message
  - `[I]` Copy Destination Port
- **Mouse support**: Click on any value row to copy

### Payment ID / Destination Port
- DERO HE uses `RPC_DESTINATION_PORT` (uint64) instead of legacy 64-char hex Payment IDs
- Integrated addresses (`deroi...`) embed port and comment in arguments
- **Send form**: Payment ID field validates as uint64 numeric (max 20 digits)
- **Integrated address detection**: Auto-fills Payment ID and Message from address
- **Visual indicator**: Shows "✓ (from address)" when port extracted from integrated address
- **Max address length**: 300 characters (integrated addresses can be up to 297-298 chars)

### Network Selection
- Shown when opening a wallet with unknown network
- Options: Mainnet (dero1...), Testnet (deto1...), Simulator (deto1...)
- Network selection is saved for future wallet opens
- Each wallet remembers its network in `~/.derotui.json`
- **Colors**: Mainnet = green, Testnet = orange, Simulator = cyan

### Wallet Operations
- **Open**: File picker for .db files
- **Create**: Generates new wallet, displays seed phrase
- **Restore from Seed**: 25-word seed phrase input with validation
- **Restore from Key**: 64-character hex key input
- **Online mode**: Calls `SetOnlineMode()` to start sync loop
- **Balance sync**: Uses DERO's encrypted balance system
- **Password change**: In-app password changing via dashboard

### Payment Request (Integrated Address)
- **Generate**: Create integrated addresses with embedded destination port and optional comment
- **QR Display**: Show QR code for generated integrated address
- **Copy**: Copy integrated address to clipboard
- **Auto-fill**: When sending to integrated addresses, destination port and comment are auto-extracted

### XSWD (Cross-Site Wallet Daemon) Server
- **WebSocket Protocol**: Allows dApps to connect on port 44326 via `/xswd` endpoint
- **User Control**: Toggle server on/off from dashboard (key `X`)
- **App Authorization**: Dialog shows connecting app's name, description, and URL for user approval
- **Permission System**: Per-method permission requests with options:
  - Allow (this request only)
  - Deny (this request only)
  - Always Allow (all future requests for this method)
  - Always Deny (all future requests for this method)
- **Callback Bridge**: XSWD callbacks block on channels while TUI displays dialogs, responses sent back through channels

### Selection Styles (Unified)
Two categories of selection across the app:

**A. List/Menu Selection**
- Selected: Neon purple `▸` prefix + white bold text
- Unselected: No prefix + muted gray text
- Used in: Welcome command menu, Restore submenu, Network selection

**B. Form Element Focus**
- Border color changes from `ColorBorder` to `ColorPrimary`
- Used in: Send form fields, input boxes

**C. Table Row Selection**
- Full-row purple background + yellow text
- Used in: Transaction history

## Architecture

### Bubble Tea Model
```go
type Model struct {
    page       Page           // Current page enum
    wallet     *wallet.Wallet // Wallet instance
    // ... page models and state
}
```

### Pages
```go
const (
    PageWelcome Page = iota    // 0
    PageFilePicker             // 1
    PagePassword               // 2
    PageNetwork                // 3 - Network selection
    PageSeed                   // 4
    PageKeyInput               // 5
    PageQRCode                 // 6 - QR code display
    PageMain                   // 7 - Dashboard
    PageSend                   // 8 - Send form
    PageHistory                // 9 - Transaction history
    PageTxDetails              // 10 - Transaction details
    PageDaemon                 // 11 - Daemon connection
    PageIntegratedAddr         // 12 - Generate integrated address
    PageXSWDAuth               // 13 - XSWD app authorization
    PageXSWDPerm               // 14 - XSWD permission request
)
```

### Message Types
- `tickMsg` - Periodic updates (5 second interval)
- `daemonStatusMsg` - Daemon connection status
- `startupCheckMsg` - Last wallet check on startup
- `walletOpenedMsg` - Wallet open result
- `walletCreatedMsg` - Wallet creation result
- `walletRestoredMsg` - Wallet restore result
- `transferResultMsg` - Transfer result
- `passwordChangedMsg` - Password change result
- `walletDaemonConnectedMsg` - Async daemon connection result
- `networkRequiredMsg` - Wallet needs network selection
- `daemonConnectMsg` - Daemon connection attempt result

### Key Bindings (Dashboard)
- `S` - Send DERO
- `C` - Copy address
- `Y` - QR Code
- `R` - Payment Request (Integrated Address)
- `V` - View seed phrase
- `K` - View hex key
- `H` - History
- `P` - Change password
- `X` - Toggle XSWD server
- `G` - Register wallet (when unregistered)
- `←/→` or `h/l` - Switch action pages
- `Esc` - Close wallet, return to welcome
- `Q` - Quit application
- `Ctrl+C` - Force quit

### Key Bindings (Transaction Details)
- `Esc` - Close details, return to history
- `T` - Copy TxID
- `B` - Copy Block Hash
- `H` - Copy Block Height
- `S` - Copy Sender
- `D` - Copy Destination
- `P` - Copy Proof
- `M` - Copy Message
- `I` - Copy Destination Port

### Mouse Support
- All pages implement `HandleMouse(msg tea.MouseMsg, width, height int) bool`
- Dashboard: Click quick actions, click page indicators, click transactions
- History: Click to select, double-click for details
- Transaction Details: Click rows to copy
- Send form: Click fields, buttons
- QR Code: Click anywhere to close
- Network selection: Click options
- Welcome: Click menu items
- Daemon selection: Double-click to confirm
- XSWD dialogs: Click buttons

## Code Style Guidelines

### Copyright Headers
All source files should include:
```go
// Copyright 2017-2026 DERO Project. All rights reserved.
```

### Imports
Use grouped import blocks:
```go
import (
    "fmt"
    "strings"

    "github.com/atotto/clipboard"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)
```

### Component Pattern
Each page/component follows this pattern:
```go
type PageModel struct {
    // state fields
}

func NewPage() PageModel { ... }
func (p PageModel) Init() tea.Cmd { ... }
func (p PageModel) Update(msg tea.Msg) (PageModel, tea.Cmd) { ... }
func (p PageModel) View() string { ... }
```

### Error Handling
- Store `currentError` at start of Update
- Clear error on key press (user typing)
- Preserve error for non-KeyMsg (cursor blink, etc.)

### Styling
- Use `styles.TitleStyle`, `styles.MutedStyle`, etc. from styles package
- Use `lipgloss.JoinVertical()` for composing views
- Use `styles.BoxStyle` for bordered containers
- **CRITICAL**: `lipgloss.Width()` and centering functions don't properly handle ANSI escape codes. The escape codes are counted as part of string length, causing truncation and misalignment.

**Reliable pattern for centered/styled text:**
1. Calculate visible text length manually
2. Pad with plain spaces to target width
3. Apply lipgloss styles to the already-padded string
4. Concatenate with `strings.Join` or `+` (NOT `JoinHorizontal` for multi-colored segments)

## CLI Flags

```bash
--wallet-file <file>              Wallet file path
--password <password>             Wallet password
--offline                         Run in offline mode
--offline-datafile <file>         Offline data file (default "getoutputs.bin")
--rpc-server                      Enable RPC server
--rpc-bind <addr>                 RPC bind address (default "127.0.0.1:20209")
--rpc-login <user:pass>           RPC credentials
--allow-rpc-password-change       Allow RPC password change via RPC
--testnet                         Use testnet
--simulator                       Connect to simulator (port 20000)
--daemon-address <addr>           Daemon address (default "localhost:10102")
--socks-proxy <ip:port>           SOCKS proxy
--generate-new-wallet             Generate new wallet
--restore-deterministic-wallet    Restore from seed
--electrum-seed <seed>            Recovery seed
--unlocked                        Keep wallet unlocked
--debug                           Enable debug logging
```

## DERO Wallet API Integration

### Key Functions Used
- `walletapi.Open_Encrypted_Wallet()` - Open existing wallet
- `walletapi.Create_Encrypted_Wallet_Random()` - Create new wallet
- `walletapi.Create_Encrypted_Wallet_From_Recovery_Words()` - Restore from seed
- `walletapi.Create_Encrypted_Wallet()` - Restore from key
- `wallet.SetDaemonAddress()` - Set daemon endpoint
- `wallet.SetOnlineMode()` - Enable sync loop
- `wallet.Get_Balance()` - Get wallet balance
- `wallet.GetSeed()` - Get seed phrase
- `wallet.Get_Keys()` - Get private keys
- `walletapi.Connect()` - Establish WebSocket connection
- `walletapi.Keep_Connectivity()` - Background connection maintenance

### Global Variables
- `walletapi.Daemon_Endpoint_Active` - Active daemon address used by Keep_Connectivity
- `walletapi.Connected` - Connection status flag

## Navigation

### Welcome Page
- Type `/` to open command menu
  - `/open` - Open an existing wallet
  - `/create` - Create a new wallet
  - `/restore` - Restore wallet (submenu: From Seed, From Key)
  - `/connect` - Connect to a daemon
  - `/exit` - Exit the application
- Arrow keys to navigate menu
- Enter to select
- Type command name for autocomplete

### Main Dashboard
- **Page 1 - Core:**
  - `S` - Send DERO
  - `C` - Copy address
  - `Y` - QR Code
  - `H` - History
  - `G` - Register wallet (when unregistered)
- **Page 2 - Advanced:**
  - `V` - View seed phrase
  - `K` - View hex key
  - `P` - Change password
  - `R` - Payment Request (Integrated Address)
  - `X` - Toggle XSWD server
- `←/→` or `h/l` - Switch action pages
- Mouse click on page indicators or quick actions
- `Esc` to close wallet
- `Q` to quit

### Transaction History
- Arrow keys to navigate
- Enter to view details
- Mouse click to select
- Mouse double-click for details
- Esc to go back

### Transaction Details
- `T` - Copy TxID
- `B` - Copy Block Hash
- `H` - Copy Block Height
- `S` - Copy Sender
- `D` - Copy Destination
- `P` - Copy Proof
- `M` - Copy Message
- `I` - Copy Destination Port (Payment ID)
- `Esc` to go back
- Mouse click on any value row to copy

### Send Form
- Tab/Shift+Tab to switch fields
- Arrow keys for ring size selection
- Enter to send (when valid)
- Esc to cancel

### Payment Request (Integrated Address)
- Tab to switch fields
- Enter to generate
- `Y` to show QR code
- `C` to copy integrated address
- Esc to close

### XSWD Authorization Dialog
- `A` - Accept connection
- `R` - Reject connection
- Tab to switch between buttons
- Enter to confirm
- Esc to reject

### XSWD Permission Dialog
- `1` - Allow (this request only)
- `2` - Deny (this request only)
- `3` - Always Allow (all future requests for this method)
- `4` - Always Deny (all future requests for this method)
- ↑/↓ to navigate
- Enter to confirm
- Esc to deny

### Forms/Dialogs
- `Enter` to confirm
- `Esc` to cancel/go back
- `Tab` to switch fields (password creation)
- `C` to copy (seed/key/QR display)

## Security Considerations

- Never log passwords or seeds
- Wallet files handled securely via DERO's encrypted wallet API
- Password input uses masked characters
- Seed/key display includes warning about not sharing
- Confirmation required for sensitive operations

## Technical Notes

### Lipgloss Width and ANSI Codes
When using `lipgloss.Width()` or `lipgloss.Center()` on strings containing ANSI escape codes (styled/colored text), the escape codes are included in the character count. This causes:
- Incorrect width calculations
- Truncation of visible text
- Misalignment of columns

**Solution for the Seed Card Grid:**
```go
// Calculate visible text length manually
visibleLen := len(word)
// Pad with plain spaces
padding := strings.Repeat(" ", cardWidth-visibleLen)
// Apply styles to padded string
styledWord := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(word + padding)
```

### Custom Table Implementation
The transaction history uses a custom implementation rather than `bubbles/table` to achieve:
- Full-row selection highlighting (purple background, yellow text)
- Color-coded transaction types (green for IN, red for OUT)
- Manual column alignment without lipgloss.Width on styled text
- Control over scroll behavior and cursor position

### Message Payload Trimming
DERO wallet API pads payloads with zeros to 144 bytes. CBOR decoder fails with "extraneous data" error.
**Fix:** Trim trailing zeros from payload before CBOR unmarshaling:
```go
payload = bytes.TrimRight(payload, "\x00")
```

### Integrated Addresses
Integrated addresses embed arguments (RPC_DESTINATION_PORT, RPC_COMMENT) in the address itself. When parsing:
1. Detect with `addr.IsIntegratedAddress()`
2. Extract arguments with `addr.Arguments`
3. Validate with `arguments.Validate_Arguments()`
4. Copy ALL embedded arguments (not just destination port)
