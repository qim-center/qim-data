# Using Magic Wormhole with QIM Relay/Transit

This guide shows how to use `magic-wormhole` with the QIM-hosted servers:

- Relay (mailbox): `wss://wormhole-mailbox.qim.dk/v1`
- Transit helper: `tcp:wormhole-transit.qim.dk:4001`

## 1) Install magic-wormhole

Install with `pip` (user-local install):

```bash
python3 -m pip install --user --upgrade magic-wormhole
```

If you prefer a virtual environment:

```bash
python3 -m venv .venv
source .venv/bin/activate
python3 -m pip install --upgrade pip magic-wormhole
```

Verify installation:

```bash
wormhole --version
```

## 2) Send a file

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 --transit-helper=tcp:wormhole-transit.qim.dk:4001 send /path/to/file.dat
```

You will get a one-time code, for example:

```text
Wormhole code is: 7-distortion-mohawk
```

Share that code with the receiver over a separate channel.

## 3) Receive a file

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 receive
```

Then enter the code when prompted.

You can also pass the code directly:

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 receive 7-distortion-mohawk
```

Note: in some `wormhole` versions, `--transit-helper` must be given as a global option before `send` (as shown above).

## 4) Quick troubleshooting

- If send/receive fails with relay errors, verify URL spelling: `wss://wormhole-mailbox.qim.dk/v1`
- If transfers fail only for some networks, direct P2P may be blocked; transit fallback should use `wormhole-transit.qim.dk:4001`
- Check local version/options: `wormhole --help`
