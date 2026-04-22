# `qim-data` CLI (MVP v0)

This document describes the first wrapper implementation around `croc`.

## Why this exists

`croc` is the transfer engine. `qim-data` provides stable defaults so users do not need to remember relay host, relay password flags, or version requirements.

In Python/Bash terms:

- `croc` is the worker executable.
- `qim-data` is a thin orchestrator script, but implemented in Go as a single portable binary.

## Commands

## `qim-data setup`

Stores local config in the user config directory:

- Linux: `~/.config/qim-data/config.json`
- macOS: `~/Library/Application Support/qim-data/config.json`
- Windows: `%AppData%\qim-data\config.json`

Config fields:

- `relay` (default: `data-relay.qim.dk:9009`)
- `relay_pass` (relay secret)
- `croc_path` (optional fixed binary path)

Behavior:

- If no valid `croc` v10+ is found, setup auto-downloads pinned `croc` (`v10.4.2`) for the current OS/arch.
- Downloaded binary is stored in a managed user path and written to config as `croc_path`.
  - Linux example path: `~/.cache/qim-data/bin/croc`

Example:

```bash
qim-data setup

# optional advanced usage
qim-data setup --relay data-relay.qim.dk:9009 --pass-file ~/.config/qim-data/relay.pass
```

## `qim-data send`

Wraps:

```bash
croc --relay <relay> send ...
```

`qim-data` injects relay credentials through environment variables, and redacts the configured relay secret from command output.

Example:

```bash
qim-data send ./dataset.zarr
qim-data send --code my-custom-code ./bigfile.tif
```

Pass through additional `croc send` flags after `--`:

```bash
qim-data send ./dataset -- --no-local --transfers 2
```

## `qim-data receive`

Wraps:

```bash
croc --relay <relay>
```

If a code is provided, `qim-data` sets `CROC_SECRET=<code>` so Linux/macOS users avoid classic-mode friction.
If no code is provided, `qim-data` prompts for it and still passes it as `CROC_SECRET`.

Examples:

```bash
qim-data receive
qim-data receive 1234-code-words
qim-data receive --out /data/incoming
```

## `qim-data doctor`

Checks:

- config presence
- relay configured
- relay password configured
- croc binary discovery
- croc version (requires major `v10+`)
- relay TCP reachability

Example:

```bash
qim-data doctor
```

## Current limitations (intentional for MVP)

- Relay password is stored in local config as plain text for now.
- No enterprise auth layer yet; relay security is currently password + network controls.

## Next planned improvements

1. OS-native secure storage for relay secret.
2. Additional diagnostics (`qim-data doctor --verbose`).
3. Packaging/release pipeline for Linux/macOS/Windows.
