# Using Magic Wormhole with QIM Relay/Transit

This guide shows how to use `magic-wormhole` with the current QIM-hosted configuration:

- Relay (mailbox): `wss://wormhole-mailbox.qim.dk/v1`
- Transit helper: `tcp:wormhole-transit.qim.dk:443`

Server placement:

- Mailbox host: `qimserver.compute.dtu.dk` (`130.225.68.197`)
- Transit host: `comp-vmfima.compute.dtu.dk` (`130.225.69.184`)

## Install magic-wormhole

Install with `pip` (user-local install):

```bash
pip install --upgrade magic-wormhole
```

Verify installation:

```bash
wormhole --version
```

## Send a file

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 --transit-helper=tcp:wormhole-transit.qim.dk:443 send /path/to/file.dat
```

You will get a one-time code, for example:

```text
Wormhole code is: 7-distortion-mohawk
```

Share that code with the receiver over a separate channel.

## Receive a file

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 receive
```

Then enter the code when prompted.
