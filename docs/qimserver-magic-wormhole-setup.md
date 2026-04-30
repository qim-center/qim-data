# qimserver Magic Wormhole Implementation Notes

Date: 2026-04-30
Primary host: `qimserver.compute.dtu.dk`

This document records the implemented and validated split deployment.

# Magic-Wormhole Deployment Strategy

## Current State (Implemented)

Production is now split across two hosts:

| Component | Hostname | FQDN | Public IP | Endpoint |
|---|---|---|---|---|
| Mailbox relay | `qimserver` | `qimserver.compute.dtu.dk` | `130.225.68.197` | `wss://wormhole-mailbox.qim.dk/v1` |
| Transit relay | `comp-vmfima` | `comp-vmfima.compute.dtu.dk` | `130.225.69.184` | `tcp:wormhole-transit.qim.dk:443` |

Reason for split:

- HPC reachability to transit on `:4001` was not reliable.
- `qimserver` has shared nginx on `:443`, so raw transit could not be colocated there.
- Dedicated VM now serves transit on `:443`, and this path is validated.

Validated behavior:

- Same-network peers use direct P2P (transit bypassed).
- HPC -> external hotspot uses private transit: `relay:tcp:wormhole-transit.qim.dk:443`.

# Set-up

## Goal

Deploy private Wormhole infrastructure so transfers can use:

- Mailbox relay (WebSocket handshake): `wss://wormhole-mailbox.qim.dk/v1`
- Transit helper (TCP fallback/data relay): `tcp:wormhole-transit.qim.dk:443`

## Final Deployed Topology

- Mailbox host: `qimserver.compute.dtu.dk` (`130.225.68.197`)
- Transit host: `comp-vmfima.compute.dtu.dk` (`130.225.69.184`)
- Transit VM hostname: `comp-vmfima`
- Transit VM FQDN: `comp-vmfima.compute.dtu.dk`

## DNS Final State

Verified A records:

- `wormhole-mailbox.qim.dk` -> `130.225.68.197`
- `wormhole-transit.qim.dk` -> `130.225.69.184`

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

## qimserver Services

Installed and used on `qimserver`:

- `/etc/systemd/system/wormhole-mailbox.service`

Service behavior:

- Mailbox binds loopback only: `127.0.0.1:4000`

Enabled and started:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now wormhole-mailbox.service
```

Mailbox service reached `active (running)`.

## Transit Service on Dedicated VM

Transit runs on a dedicated VM to provide a reachable `:443` endpoint for HPC networks.

- Hostname: `comp-vmfima`
- FQDN: `comp-vmfima.compute.dtu.dk`
- Public IP: `130.225.69.184`
- Service: `/etc/systemd/system/wormhole-transit.service`
- Bind port: `443`

Key service details on VM:

- `ExecStart=... transitrelay --port=tcp:443 ...`
- `AmbientCapabilities=CAP_NET_BIND_SERVICE`
- `CapabilityBoundingSet=CAP_NET_BIND_SERVICE`

## SELinux and Firewall

Applied SELinux settings on `qimserver`:

```bash
sudo semanage port -a -t http_port_t -p tcp 4000
sudo setsebool -P httpd_can_network_connect 1
sudo semanage fcontext -a -t var_t "/var/lib/wormhole(/.*)?"
sudo restorecon -Rv /var/lib/wormhole
```

Applied on transit VM (`comp-vmfima`) for port `443`:

```bash
sudo restorecon -Rv /opt/wormhole-venv
sudo semanage fcontext -a -t var_t "/var/lib/wormhole(/.*)?"
sudo restorecon -Rv /var/lib/wormhole
```

Notes:

- No AVC denials were observed during startup/tests on final topology.

Firewall:

```bash
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --reload
```

Transit VM serves on `443/tcp`.

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

### Sender (HPC)

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 --transit-helper=tcp:wormhole-transit.qim.dk:443 send shell_225x128x128.tif
```

Observed:

- Transfer completed successfully.
- In constrained-network test, transfer explicitly used `relay:tcp:wormhole-transit.qim.dk:443`.

### Receiver (mobile hotspot)

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 receive
```

Observed:

- Received file successfully.
- Output confirmed use of `relay:tcp:wormhole-transit.qim.dk:443`.

### Path confirmation from transfer output

- Same-network test showed direct peer path (expected).
- HPC -> mobile hotspot test showed transit relay path on both ends: `relay:tcp:wormhole-transit.qim.dk:443`.

Interpretation:

- Mailbox relay (`wss://wormhole-mailbox.qim.dk/v1`) handled code exchange/coordination.
- Direct P2P is used when routable.
- Fallback transit relay is now served privately via `wormhole-transit.qim.dk:443` on `comp-vmfima.compute.dtu.dk`.

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
sudo ss -tlnp | grep -E '4000|443'
```

nginx check/reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
```
