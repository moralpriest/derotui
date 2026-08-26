// Copyright 2017-2026 DERO Project. All rights reserved.

# Daemon Manager Plan

## Goal

Add a `/daemon` workflow to `derotui` so the TUI can:

- reuse the existing remote/manual daemon connect flow
- start and manage a local `derod` process as a separate binary
- show local daemon runtime status and logs
- configure local daemon launch parameters from the TUI

This intentionally uses a separate managed `derod` process instead of embedding daemon code in-process.

## Architecture Decision

The first implementation uses a separate managed `derod` binary.

Reasons:

- keeps daemon crashes isolated from the TUI
- keeps upstream daemon compatibility simpler
- still provides a unified user experience through one TUI
- allows `derotui` to supervise daemon lifecycle while using `derod` RPC for live node status

## `/daemon` Command Structure

Top-level slash command:

- `/daemon`

Submenu items:

- `Connect`
- `Start Local`
- `Status`
- `Logs`
- `Settings`

The existing top-level `/connect` command remains temporarily and will be folded into `/daemon` later.

## Status Model

Daemon status comes from two sources.

### 1. Process Manager State

Managed by a new internal daemon service layer.

Fields include:

- running/stopped
- managed/external
- pid
- start time
- exit error
- binary path
- launch args
- captured stdout/stderr

### 2. `derod` RPC State

Live node health should come from the daemon API, reusing existing helpers where possible.

Fields include:

- reachable
- healthy
- synced/syncing
- network
- block height
- topo height

The UI should merge both views so it can distinguish:

- stopped
- starting
- running but RPC unavailable
- syncing
- synced
- crashed

## Local Daemon Settings

Settings are state-aware and divided into three tiers.

### Core

- Binary Path
- Network
- Data Directory
- Fast Sync
- Integrator Address
- Node Tag
- RPC Bind
- P2P Bind
- GetWork Bind
- SOCKS Proxy
- Debug

### Advanced

- Time In Sync
- Sync Node
- Min Peers
- Max Peers
- Log Dir

### Expert

- Priority Nodes
- Exclusive Nodes
- Console Log Level
- File Log Level
- Prune History

## UX Decisions

### Settings Interaction Model

Use a mixed interaction model:

- inline toggles/selectors for simple booleans and enums
- focused editors for paths, binds, addresses, tags, and node lists

### Integrator Address

- do not auto-fill silently
- offer an explicit `Use current wallet address` action
- only set the field when the user chooses that action

### Node Tag

- use a hostname-based placeholder suggestion
- do not save it unless the user explicitly sets it

### Fast Sync

- treat `--fastsync` as a core first-run setting
- keep it prominent on the settings page

### Network Mismatch on `Start Local`

If local daemon settings use a different network than the app's current network, prompt with:

- `Switch network and start`
- `Cancel`

Do not offer `Start anyway` in the first version.

## Milestone 1 Scope

Milestone 1 assumes `derod` is already installed and accessible through a configured binary path.

Included:

- `/daemon` submenu
- reuse of existing connect flow
- real `Status` page
- real `Logs` page
- real `Settings` page
- managed local `derod` start/stop/restart
- log capture
- status polling through `derod` RPC

Not included:

- auto-download/install of `derod`
- automatic wallet switch to the managed local daemon
- top-level `/connect` removal
- `/miner`

## Proposed Packages and Files

### New Service Layer

- `internal/services/daemon/types.go`
- `internal/services/daemon/manager.go`
- `internal/services/daemon/command.go`
- `internal/services/daemon/detect.go`

### New UI Pages

- `internal/ui/pages/daemon_status.go`
- `internal/ui/pages/daemon_logs.go`
- `internal/ui/pages/daemon_settings.go`

### Existing Files Expected to Change

- `internal/config/config.go`
- `internal/ui/app.go`
- `internal/ui/app_handlers.go`
- `internal/ui/app_messages.go`
- `internal/ui/pages/welcome.go`

## Implementation Order

1. Add `/daemon` submenu and routing.
2. Extend config with local daemon launch settings.
3. Add the daemon manager service layer.
4. Implement the daemon settings page.
5. Wire `Start Local` to the manager.
6. Implement the status page.
7. Implement the logs page.
8. Integrate periodic process and RPC status refresh.
9. Keep `/connect` as a temporary alias.

## Milestone 2

Potential next steps after milestone 1:

- auto-download/install `derod`
- auto-prefer the managed local daemon for wallet connectivity
- remove top-level `/connect`
- add dashboard daemon indicator
- add `/miner`
