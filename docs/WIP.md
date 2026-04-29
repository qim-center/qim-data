# WIP: Magic Wormhole Deployment Status (Stop Point)

Date: 2026-04-29

This note records exactly where the setup ended today and what is still unresolved.

## Current Working State

- Mailbox service is running on `qimserver` via systemd.
- Transit service is running on `qimserver` via systemd.
- DNS is set:
  - `wormhole-mailbox.qim.dk` -> `130.225.68.197`
  - `wormhole-transit.qim.dk` -> `130.225.68.197`
- End-to-end transfers work.

## Important Current Port Reality

- `wormhole-transit.service` is configured with:

```text
ExecStart=... transitrelay --port=tcp:4001 ...
```

- So transit is actually listening on `4001`, not `443`.
- `nginx` is listening on `443`.

Verification done:

- `sudo ss -lntp | grep ':443'` showed `nginx` owns port `443`.
- `sudo systemctl cat wormhole-transit.service` showed transit on `4001`.

## Why Public Transit Was Used

In recent tests, sender/receiver outputs showed:

```text
...relay:tcp:transit.magic-wormhole.io:4001
```

Reason:

- Clients were told to use `--transit-helper=tcp:wormhole-transit.qim.dk:443`,
- but our transit relay is not on `443` (it is on `4001`),
- so clients fell back to public transit.

Additionally, HPC reachability test showed private transit on `4001` is not reachable from HPC.

## Mailbox URL Mode During Tests

For the 80/443 split experiment, mailbox was tested with:

- `ws://wormhole-mailbox.qim.dk/v1` (non-TLS websocket)

`wss://...` failed in that mode as expected.

## Security Note

- Using `ws://` for mailbox removes TLS transport protection for signaling metadata.
- File content remains end-to-end encrypted by Magic Wormhole.

## Open Problem To Resolve Tomorrow

Need a topology where HPC can use private transit instead of public fallback.

Likely clean options:

1. Use an additional public IP/VM so one endpoint can serve transit on `443` while mailbox stays on secure `wss` (`443`) elsewhere.
2. Keep current host but accept that HPC may fall back to public transit when private transit port is unreachable.

## Quick Resume Checklist

1. Re-check who owns key ports:
   - `sudo ss -lntp | grep -E ':80|:443|:4001'`
2. Re-check service configs:
   - `sudo systemctl cat wormhole-mailbox.service`
   - `sudo systemctl cat wormhole-transit.service`
3. Re-test from HPC reachability:
   - TCP connect test to `wormhole-transit.qim.dk` on target port.
4. Align client flags with actual deployed ports.
5. Run transfer and confirm path lines show desired relay/transit usage.
