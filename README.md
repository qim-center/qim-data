# qim-data

QIM Center wrapper around `magic-wormhole`.

## Current status

Initial Python package structure is in place with a CLI-first backend:

- `qim-data send <source>`
- `qim-data receive [code]`
- `qim-data config`

The current backend invokes the `wormhole` CLI with QIM defaults.
The internal architecture keeps a backend abstraction so we can later add a direct Python API backend.

## Package layout

- `pyproject.toml`: packaging metadata and `qim-data` entrypoint
- `src/qim_data/cli.py`: command parsing and dispatch
- `src/qim_data/app.py`: application orchestration
- `src/qim_data/config.py`: runtime configuration model
- `src/qim_data/constants.py`: QIM endpoint defaults
- `src/qim_data/backends/`: backend interface and CLI backend implementation
