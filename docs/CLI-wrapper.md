# qim-data CLI Wrapper Architecture

This document explains the implementation strategy used for the first `qim-data` release (`0.1.0`) and provides guidance for future contributors.

## Strategy

`qim-data` currently wraps the `wormhole` command-line tool instead of using the Magic Wormhole Python API directly.

Why this was chosen for v1:

- fastest path to a working tool with proven behavior
- low risk, since we reuse a well-tested transfer flow
- easy debugging (`qim-data` behavior can be compared directly against `wormhole`)

At the same time, the codebase is intentionally structured so we can later add a direct API backend without rewriting the CLI or command orchestration layers.

## Design goal

Provide a simple, QIM-opinionated command for users while preserving flexibility for future protocol-level customization.

User-facing defaults:

- mailbox relay: `wss://wormhole-mailbox.qim.dk/v1`
- transit helper: `tcp:wormhole-transit.qim.dk:443`
- app id: `qim.dk/qim-data`

## Current command behavior

Implemented commands:

- `qim-data send <source>`
- `qim-data receive [code]`
- `qim-data config`

Notable output customization:

- QR output is disabled on send (`--no-qr`)
- `Wormhole code is:` is rewritten as `Transfer code is:`
- the wormhole instruction block (`On the other computer, please run: ...`) is suppressed
- receive prompt is owned by `qim-data` (`Enter transfer code:`)

## Source layout and responsibilities

- `pyproject.toml`
  - package metadata and dependencies
  - publishes console entrypoint: `qim-data = qim_data.cli:main`

- `src/qim_data/__init__.py`
  - package version (`__version__`)

- `src/qim_data/constants.py`
  - QIM default relay/transit/app-id constants

- `src/qim_data/config.py`
  - `QimDataConfig` dataclass
  - `load_config()` currently returns built-in defaults (extension point for env/file config)

- `src/qim_data/cli.py`
  - argument parsing and command dispatch
  - `send`, `receive`, and `config` command definitions

- `src/qim_data/app.py`
  - `QimDataApp` orchestration layer
  - translates parsed CLI arguments into request models and delegates execution to backend

- `src/qim_data/backends/base.py`
  - backend contract (`TransferBackend` protocol)
  - request models: `SendRequest`, `ReceiveRequest`

- `src/qim_data/backends/wormhole_cli.py`
  - current backend implementation using subprocess execution of `wormhole`
  - send-path output rewriting/filtering logic
  - receive prompt handoff and execution

- `tests/test_cli.py`
  - initial parser smoke test scaffold

## Why the backend abstraction matters

The backend interface lets us keep these concerns isolated:

- UX and command semantics in `cli.py`/`app.py`
- transport implementation details in backend modules

Future API backend can be added as, for example:

- `src/qim_data/backends/wormhole_api.py`

and selected without breaking user-facing commands.

## Important implementation detail: PTY on send

Send output filtering originally used `stdout=PIPE`, which broke progress bar rendering in some cases.

Current implementation uses a pseudo-terminal (PTY) for send output so `wormhole` keeps TTY-style progress behavior while still allowing line/text filtering.

Contributor note:

- be careful when modifying send output handling
- pipe-based interception may degrade progress behavior (`tqdm` uses carriage returns)

## Packaging and release notes

- package is published on PyPI as `qim-data`
- runtime dependency includes `magic-wormhole`
- editable/local install works via `pip install -e .`

## Suggested next development steps

- add `check` command for dependency/network diagnostics
- improve tests (backend behavior, output rewriting/filtering, error paths)
- optionally add backend selection plumbing for experimental API backend

## Contributor guidance

- preserve default endpoints unless there is an explicit migration plan
- treat app-id changes as compatibility-impacting changes
- keep user-facing text intentional and stable (scripts/docs may rely on it)
- avoid introducing output handling that harms interactive UX
- keep architecture layered: CLI -> app orchestration -> backend
