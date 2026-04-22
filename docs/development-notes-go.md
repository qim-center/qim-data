# Development Notes (Go, with Python/Bash mapping)

This is a quick orientation for contributors who are stronger in Python/Bash.

## Project layout

- `cmd/qim-data/main.go`
- `internal/config/config.go`
- `internal/croc/croc.go`

Think of it like:

- `cmd/qim-data/main.go` = `main.py` or top-level bash script with subcommand parsing.
- `internal/config/config.go` = helper module handling config file read/write.
- `internal/croc/croc.go` = helper module wrapping `subprocess.run(["croc", ...])`.

## How command flow works

1. CLI reads command (`setup`, `send`, `receive`, `doctor`).
2. Loads local config (`~/.config/qim-data/config.json` on Linux).
3. Resolves `croc` binary path.
4. Builds arguments and executes `croc`.

Python equivalent pattern:

```python
cfg = load_config()
cmd = ["croc", "--relay", cfg.relay, "--pass", cfg.relay_pass, "send", path]
subprocess.run(cmd, check=True)
```

## Security note for current MVP

Right now `relay_pass` is stored in plain text in local config (permissions are restricted).

Future improvement:

- Move to OS-native credential stores:
  - Linux Secret Service / keyring
  - macOS Keychain
  - Windows Credential Manager

## Useful local commands

Build:

```bash
go build -o ./bin/qim-data ./cmd/qim-data
```

Run directly:

```bash
go run ./cmd/qim-data --help
go run ./cmd/qim-data setup --relay data-relay.qim.dk:9009 --pass-file ~/.config/qim-data/relay.pass
go run ./cmd/qim-data doctor
```

## Common debugging pattern

If wrapper command fails, run the equivalent `croc` command directly and compare behavior.

