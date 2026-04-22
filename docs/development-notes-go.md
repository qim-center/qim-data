# Development Notes (Go, with Python/Bash mapping)

This is a quick orientation for contributors who are stronger in Python/Bash.

## Project layout

- `cmd/qim-data/main.go`
- `internal/config/config.go`
- `internal/croc/croc.go`
- `internal/installer/croc_installer.go`

Think of it like:

- `cmd/qim-data/main.go` = `main.py` or top-level bash script with subcommand parsing.
- `internal/config/config.go` = helper module handling config file read/write.
- `internal/croc/croc.go` = helper module wrapping `subprocess.run(["croc", ...])`.
- `internal/installer/croc_installer.go` = helper module that downloads pinned `croc` release assets per OS/arch.

## How command flow works

1. CLI reads command (`setup`, `send`, `receive`, `doctor`).
2. `setup` ensures a usable `croc` v10+ (auto-download if needed).
3. Loads local config (`~/.config/qim-data/config.json` on Linux).
4. Resolves `croc` binary path.
5. Builds arguments and executes `croc`.

Python equivalent pattern:

```python
cfg = load_config()
env = dict(os.environ)
if cfg.relay_pass_file:
    env["CROC_PASS"] = cfg.relay_pass_file
cmd = ["croc", "--relay", cfg.relay, "send", path]
subprocess.run(cmd, check=True, env=env)
```

## Security note for current MVP

Current phase default is open relay mode (no password configured).
Optional hardening mode stores relay secret in a local file (`relay_pass_file` in config points to it), with restrictive permissions.

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
go run ./cmd/qim-data setup
go run ./cmd/qim-data doctor
```

## Common debugging pattern

If wrapper command fails, run the equivalent `croc` command directly and compare behavior.

Prompt model:

- `qim-data` should avoid replacing native transfer prompts where possible.
- Let `croc` handle receive code prompts and acceptance prompts by default.
