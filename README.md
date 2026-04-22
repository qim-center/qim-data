# qim-data

Data transfer tooling from the Qim Center.

## Current status

- Relay strategy and setup runbook are in place.
- Initial Go wrapper CLI scaffold (`qim-data`) is implemented.
- `qim-data setup` now auto-installs pinned `croc` binaries when needed.
- Target onboarding flow: `qim-data setup` once, then `qim-data send` / `qim-data receive`.
- Current phase default: open relay mode (no password required).

See:

- [strategy.md](strategy.md)
- [docs/qimserver-relay-runbook.md](docs/qimserver-relay-runbook.md)
- [docs/qim-data-cli.md](docs/qim-data-cli.md)
- [docs/development-notes-go.md](docs/development-notes-go.md)
