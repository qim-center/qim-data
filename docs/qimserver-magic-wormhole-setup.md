# qimserver Magic Wormhole Implementation Notes

Date: 2026-04-29
Host: `qimserver.compute.dtu.dk`

This document records the exact implementation and validation steps completed for self-hosted Magic Wormhole relay/transit on `qimserver`.

## Goal

Deploy private Wormhole infrastructure so transfers can use:

- Mailbox relay (WebSocket handshake): `wss://wormhole-mailbox.qim.dk/v1`
- Transit helper (TCP fallback/data relay): `tcp:wormhole-transit.qim.dk:4001`

## DNS Final State

Verified A records:

- `wormhole-mailbox.qim.dk` -> `130.225.68.197`
- `wormhole-transit.qim.dk` -> `130.225.68.197`

Notes:

- A wrong `transit` DNS record and IPv6 answer were encountered initially and corrected.
- Final `getent hosts` checks returned the expected IPv4 for both names.

## System User and Data Directory

Created dedicated service account and data directory:

```bash
sudo useradd -r -s /sbin/nologin -d /var/lib/wormhole -m wormhole
sudo mkdir -p /var/lib/wormhole
sudo chown wormhole:wormhole /var/lib/wormhole
sudo chmod 750 /var/lib/wormhole
```

Verified:

- `id wormhole` exists
- `/var/lib/wormhole` permissions/ownership correct (`wormhole:wormhole`, `750`)

## Python Environment and Packages

Created isolated venv and installed required services:

```bash
sudo python3 -m venv /opt/wormhole-venv
sudo /opt/wormhole-venv/bin/pip install --upgrade pip
sudo /opt/wormhole-venv/bin/pip install magic-wormhole-mailbox-server magic-wormhole-transit-relay
```

Verified:

- `/opt/wormhole-venv/bin/twist`
- `/opt/wormhole-venv/bin/twistd`

SELinux context restore on venv:

```bash
sudo restorecon -Rv /opt/wormhole-venv
```

## systemd Services

Installed two units:

- `/etc/systemd/system/wormhole-mailbox.service`
- `/etc/systemd/system/wormhole-transit.service`

Service behavior:

- Mailbox binds loopback only: `127.0.0.1:4000`
- Transit binds externally: `0.0.0.0:4001`

Enabled and started:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now wormhole-mailbox.service wormhole-transit.service
```

Both services reached `active (running)`.

## SELinux and Firewall

Applied SELinux settings:

```bash
sudo semanage port -a -t http_port_t -p tcp 4000
sudo semanage port -a -t unreserved_port_t -p tcp 4001
sudo setsebool -P httpd_can_network_connect 1
sudo semanage fcontext -a -t var_t "/var/lib/wormhole(/.*)?"
sudo restorecon -Rv /var/lib/wormhole
```

Notes:

- `4001/tcp` already appeared under existing labels on this shared server; wormhole still started and bound successfully.
- No AVC denials were observed during startup/tests (`ausearch -m avc -ts recent | grep -E 'wormhole|twist'` returned none).

Firewall:

```bash
sudo firewall-cmd --permanent --add-port=4001/tcp
sudo firewall-cmd --reload
```

Port `4001/tcp` is open.

## nginx + TLS for Mailbox

Certificate issued with Certbot for `wormhole-mailbox.qim.dk` and deployed into nginx.

An HTTPS redirect loop (`301` on `/v1`) was diagnosed and fixed by replacing the vhost with explicit proxy locations for `/v1` and `/v1/`.

Final vhost path:

- `/etc/nginx/conf.d/wormhole-mailbox.qim.dk.conf`

Final behavior:

- Port 80 redirects to HTTPS
- HTTPS server proxies WebSocket endpoint path `/v1` to `http://127.0.0.1:4000/v1`
- Non-Wormhole paths return `404`

## End-to-End Validation (Successful)

### Sender (local workstation)

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 --transit-helper=tcp:wormhole-transit.qim.dk:4001 send mussel.tif
```

Observed:

- 2.40 GB file transfer completed successfully
- Throughput around `116 MB/s`
- Confirmation received

### Receiver (HPC)

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 receive
```

Observed:

- Received file `mussel.tif` successfully
- Transfer completed at matching speed

### Path confirmation from transfer output

- Sender showed direct peer path: `Sending (<-192.38.95.134:50168)..`
- Receiver showed direct peer path: `Receiving (->tcp:10.52.16.250:41851)..`

Interpretation:

- Mailbox relay (`wss://wormhole-mailbox.qim.dk/v1`) handled code exchange/coordination.
- Data path for this specific transfer was direct P2P between endpoints (fast path), which is expected when routable.
- Transit relay remains available for NAT/firewall fallback via `wormhole-transit.qim.dk:4001`.

## Useful Operational Commands

Service status:

```bash
sudo systemctl status wormhole-mailbox.service --no-pager -l
sudo systemctl status wormhole-transit.service --no-pager -l
```

Logs:

```bash
sudo journalctl -u wormhole-mailbox -u wormhole-transit -f
```

Listening sockets:

```bash
sudo ss -tlnp | grep -E '4000|4001'
```

nginx check/reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

## Recommended Client Configuration

For any sender/receiver that must use this infrastructure, set:

```bash
export WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
export WORMHOLE_TRANSIT_HELPER=tcp:wormhole-transit.qim.dk:4001
```

Note: some `wormhole` versions accept `--transit-helper` only as a global option (before subcommands).
