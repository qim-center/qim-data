# `qim-data` CLI (MVP v0)

This document describes the first wrapper implementation around `croc`.

## Why this exists

`croc` is the transfer engine. `qim-data` provides stable defaults so users do not need to remember relay host or version requirements.

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
- `relay_pass_file` (optional path to local relay secret file)
- `croc_path` (optional fixed binary path)

Behavior:

- If no valid `croc` v10+ is found, setup auto-downloads pinned `croc` (`v10.4.2`) for the current OS/arch.
- Downloaded binary is stored in a managed user path and written to config as `croc_path`.
  - Linux example path: `~/.cache/qim-data/bin/croc`
- If `--pass` or `--pass-file` is provided, relay secret is stored in a local secret file (Linux example: `~/.config/qim-data/relay.pass`).
- If no password is provided, setup configures open relay mode.

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

If `relay_pass_file` is configured, `qim-data` injects relay credentials using `CROC_PASS=<path-to-secret-file>`.

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
If no code is provided, `croc` handles interactive code entry and transfer prompts.

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
- relay password status (configured or open relay mode)
- croc binary discovery
- croc version (requires major `v10+`)
- relay TCP reachability

Example:

```bash
qim-data doctor
```

## Current limitations (intentional for MVP)

- No enterprise auth layer yet.
- In open relay mode (default in this phase), relay abuse protection relies on monitoring and network controls.

## Next planned improvements

1. OS-native secure storage for relay secret.
2. Additional diagnostics (`qim-data doctor --verbose`).
3. Packaging/release pipeline for Linux/macOS/Windows.
