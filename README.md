# DERO Wallet TUI

A modern terminal-based wallet for the DERO blockchain, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **Modern TUI**: Clean, keyboard-driven interface using Bubble Tea
- **Wallet Management**: Create, open, and restore wallets from seed or hex key
- **Auto-Open**: Automatically reopens your last wallet on startup with password prompt
- **CLI Startup Flows**: Supports direct create/restore startup and optional password auto-open
- **Real-time Updates**: Balance and sync status update automatically
- **Send/Receive**: Transfer DERO with integrated address and destination port support
- **Payment Requests**: Generate integrated addresses with embedded payment details
- **Transaction History**: View all transactions with color-coded types and full details
- **QR Codes**: Generate QR codes for wallet addresses and integrated addresses
- **XSWD Server**: Built-in WebSocket server for dApp integration with user-controlled permissions
- **Settings**: Change password, view seed/key, rescan blockchain
- **RPC Server**: Built-in JSON-RPC server for external access
- **Offline Mode**: Work without daemon connection
- **Network Support**: Mainnet, Testnet, and Simulator support with auto-detection

## Installation

### One-liner install (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/moralpriest/derotui/main/scripts/install.sh | bash
```

### Build from source

```bash
go build -o derotui ./cmd/
```

To launch from any terminal, place the binary in your PATH (for example):

```bash
install -m 0755 derotui "$HOME/.local/bin/derotui"
```

### Windows (PowerShell)

Download and run the latest Windows release:

```powershell
$repo = "moralpriest/derotui"
$api  = "https://api.github.com/repos/$repo/releases/latest"
$rel  = Invoke-RestMethod -Uri $api
$asset = $rel.assets | Where-Object { $_.name -like "*windows-amd64.exe" } | Select-Object -First 1
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile "derotui.exe"
.\derotui.exe
```

Optional: move `derotui.exe` to a folder in your `PATH` (for example `%USERPROFILE%\bin`).

### Development tasks

```bash
task ci
task security
task build
```

## Usage

```bash
# Open wallet (interactive)
./derotui

# With specific wallet file
./derotui --wallet-file mywallet.db

# Open wallet directly with password (no prompt)
./derotui --wallet-file mywallet.db --password "your-password"

# Offline mode
./derotui --offline

# Enable RPC server
./derotui --rpc-server

# Create new wallet
./derotui --generate-new-wallet

# Restore from seed
./derotui --restore-deterministic-wallet --electrum-seed "word1 word2 ..."

# Restore from seed interactively (prompts for seed in TUI)
./derotui --restore-deterministic-wallet

# Testnet
./derotui --testnet

# Simulator (local development)
./derotui --simulator
```

## Important Limitations

### Network Switching Requires App Restart

**Due to architectural limitations in the underlying walletapi library, you cannot switch between different network types (Mainnet/Testnet/Simulator) without restarting the application.**

- **Mainnet wallets** (default, port 10102)
- **Testnet wallets** (port 40402) - start with `--testnet` flag
- **Simulator wallets** (port 20000) - start with `--simulator` flag

**Example workflow:**
```bash
# For mainnet wallets (default)
./derotui

# For testnet wallets - must restart with flag
./derotui --testnet

# For simulator wallets - must restart with flag
./derotui --simulator
```

If you try to open a wallet with a different network than the current connection, you'll see an error message explaining that a restart is required.

## CLI Startup Behavior

- `--generate-new-wallet` starts directly in wallet creation flow.
- `--restore-deterministic-wallet` starts restore flow:
  - With `--electrum-seed`: opens password step directly.
  - Without `--electrum-seed`: opens seed input page.
- `--wallet-file` opens that file path exactly as provided.
- `--wallet-file` + `--password` attempts direct wallet open without password prompt.
- `--offline` skips daemon auto-detection and keeps wallet in offline mode.

## Keyboard Navigation

### Welcome Page
- Type `/` to open command menu
- `/open` - Open an existing wallet
- `/create` - Create a new wallet  
- `/restore` - Restore wallet from seed or hex key
- `/themes` - Change color theme
- `/connect` - Connect to a daemon
- `/exit` - Exit the application
- Arrow keys to navigate
- Enter to select

#### Themes
Available color themes (selected theme is saved and restored on startup):
- **Neon** (default) - Cyberpunk purple aesthetic
- **Matrix** - Green phosphor terminal with semantic colors preserved
- **Amber CRT** - Warm amber-orange CRT with brown backgrounds
- **Solarized Dark** - Cool blue-cyan palette with frosty tones
- **Gruvbox Dark** - Warm retro terminal colors
- **Crimson Dark** - Deep red theme on dark maroon backgrounds
- **Neon Pink** - Hot pink neon on deep violet-black with enhanced selection contrast

### Dashboard (Main View)

**Page 1 - Core Actions:**
- `S` - Send DERO
- `C` - Copy address to clipboard
- `Y` - Show QR code
- `H` - Transaction history
- `G` - Register wallet (shown only when unregistered)

**Page 2 - Advanced Actions:**
- `V` - View seed phrase
- `K` - View hex key
- `P` - Change password
- `R` - Generate payment request (integrated address)
- `X` - Toggle XSWD server

**Navigation:**
- `←/→` or `h/l` - Switch between action pages
- `Esc` - Close wallet, return to welcome
- `Q` - Quit application

### Transaction History
- Arrow keys - Navigate transactions
- Enter - View transaction details
- Mouse click - Select transaction
- Mouse double-click - View details
- Esc - Back to dashboard

### Transaction Details
- `T` - Copy Transaction ID
- `B` - Copy Block Hash
- `H` - Copy Block Height
- `S` - Copy Sender
- `D` - Copy Destination
- `P` - Copy Proof
- `M` - Copy Message
- `I` - Copy Destination Port
- Mouse click on any value row to copy
- `Esc` - Back to history

### Send Form
- `Tab` / `Shift+Tab` - Switch between fields
- `Arrow Left/Right` - Adjust ring size
- `Enter` - Send (when valid)
- `Esc` - Cancel

### Seed/Key Display
- `C` - Copy to clipboard
- `Esc` - Close

### QR Code Display
- `C` - Copy address to clipboard
- `Esc` or click anywhere - Close

### Payment Request (Integrated Address)
- `Tab` - Switch between fields
- `Enter` - Generate integrated address
- `Y` - Show QR code for generated address
- `C` - Copy integrated address
- `Esc` - Close

### XSWD Authorization Dialog
- `A` - Accept connection
- `R` - Reject connection
- `Tab` - Switch between buttons
- `Enter` - Confirm selection
- `Esc` - Reject

### XSWD Permission Dialog
- `1` - Allow (this request only)
- `2` - Deny (this request only)
- `3` - Always Allow (all future requests for this method)
- `4` - Always Deny (all future requests for this method)
- `↑/↓` - Navigate options
- `Enter` - Confirm selection
- `Esc` - Deny

### Universal
- `Ctrl+C` - Force quit
- `Esc` - Go back / Cancel

## Mouse Support

The wallet supports mouse interaction throughout:
- Click quick actions on dashboard
- Click page indicators to switch pages
- Click transactions in history
- Double-click transactions for details
- Click any field in transaction details to copy
- Click form buttons and fields
- Click XSWD dialog buttons
- Double-click in daemon selection to confirm

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
--daemon-address <addr>           Daemon endpoint: host:port or http(s)://host[:port]
--socks-proxy <ip:port>           SOCKS proxy
--generate-new-wallet             Generate new wallet
--restore-deterministic-wallet    Restore from seed
--electrum-seed <seed>            Recovery seed
--unlocked                        Keep wallet unlocked
--debug                           Enable debug logging
```

## Configuration

The wallet automatically stores configuration in `~/.derotui.json`:
- Last opened wallet path
- Network type per wallet (Mainnet/Testnet/Simulator)
- Selected color theme

On startup, if a previous wallet exists, you'll be prompted for its password automatically.

### Daemon Auto-Detection

On startup, the wallet automatically detects running local daemons:
- If multiple daemons are detected (Mainnet, Testnet, Simulator), a selection prompt appears
- If one daemon is detected, it connects automatically
- If no daemon is detected, defaults to Mainnet settings

## Project Structure

```
cmd/
└── main.go              # Entry point for the TUI wallet

internal/
├── config/              # User configuration (last wallet, network settings)
├── ui/                  # TUI implementation
│   ├── pages/           # UI pages (dashboard, send, history, etc.)
│   ├── styles/          # Theme and styling
│   └── components/      # Reusable UI components
└── wallet/              # Wallet wrapper and types
```

## Requirements

- Go 1.26+
- Terminal with ANSI color support

## License

MIT License - see LICENSE file for details.
