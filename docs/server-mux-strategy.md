# qim-mux: WebSocket Multiplexer Strategy

**Date:** 2026-04-24  
**Repo:** `qim-center/qim-data`  
**Audience:** Implementation agent with access to the repository

---

## 1. Problem Statement

The `qim-data` relay (`croc` running on `qimserver`) uses two TCP ports:

- `9009` — control/signaling port (the one passed to `--relay`)
- `9010` — data transfer port (negotiated by the relay at runtime)

HPC cluster login nodes at Scandinavian research institutions (e.g., DTU's `gbarlogin1`) block all outbound TCP except ports 22, 80, and 443. This means users on those nodes cannot reach the relay at all, even though the relay is correctly configured and reachable from normal networks.

Croc requires a minimum of 2 ports. We cannot reduce this to one. We therefore need a way to tunnel both ports through the single outbound port that is universally allowed: **443**.

---

## 2. Solution Overview

We introduce a small component called **`qim-mux`**, which lives in the same repository under `cmd/qim-mux/`. It is a server-side WebSocket multiplexer.

The approach:

1. The `qim-data` client detects that port 9009 is unreachable (already planned in `qim-data check`).
2. Instead of connecting croc directly to the relay, `qim-data` opens a **single WebSocket connection** to `https://data-relay.qim.dk/relay` on port 443.
3. `qim-data` creates two local TCP listeners (`localhost:9009` and `localhost:9010`) and tunnels their traffic as labelled channels over that single WebSocket.
4. croc is launched with `--relay localhost:9009` and never knows any of this is happening.
5. On the server, nginx proxies `/relay` on `data-relay.qim.dk` to `qim-mux`.
6. `qim-mux` receives the WebSocket, reads channel-labelled frames, and forwards each channel's bytes to the appropriate local croc port (`127.0.0.1:9009` or `127.0.0.1:9010`).

This requires **no changes to croc** and **no changes to the relay systemd service**. It fits cleanly into the existing nginx WebSocket proxy pattern already used by Dask, JupyterHub, and others on `qimserver`.

---

## 3. Croc Port Configuration

The relay must be reconfigured to use **exactly 2 ports**: 9009 and 9010.

Update `/etc/systemd/system/qim-data-relay.service` `ExecStart` to:

```
ExecStart=/srv/qim-data/bin/croc relay --host 0.0.0.0 --ports 9009,9010
```

This matters because `qim-data` needs to know deterministically which local ports to proxy — it always sets up tunnels for 9009 (control) and 9010 (data), no more.

Update the runbook (`docs/qimserver-relay-runbook.md`) accordingly.

---

## 4. Wire Protocol (Framing)

The WebSocket carries a simple binary framing protocol. Each frame is:

```
┌──────────┬───────────────────────────────────────┐
│ 1 byte   │ N bytes                               │
│ channel  │ payload                               │
└──────────┴───────────────────────────────────────┘
```

- **Channel `0x00`** = croc control port (9009)
- **Channel `0x01`** = croc data port (9010)
- **Payload** = raw TCP bytes for that channel, arbitrary length (bounded by WebSocket frame size)

This is intentionally minimal. There is no length prefix because WebSocket frames already carry their own length. There is no version byte in the MVP — if versioning is needed later, a channel value `0xFF` can be reserved for a handshake frame.

---

## 5. Repository Layout

Add the following to the existing repo structure:

```
cmd/
  qim-data/          ← existing CLI binary
    main.go
  qim-mux/           ← NEW: server-side mux binary
    main.go

internal/
  config/            ← existing
  croc/              ← existing
  installer/         ← existing
  tunnel/            ← NEW: shared tunnel logic used by qim-data client
    tunnel.go
```

The `internal/tunnel` package is used by `qim-data` (client side). `qim-mux` is self-contained and does not import `internal/tunnel`.

---

## 6. `qim-mux` Server Binary (`cmd/qim-mux/main.go`)

### Responsibilities

- Listen on a local port (default `127.0.0.1:9099`, configurable via `--addr`)
- Accept WebSocket connections (using `golang.org/x/net/websocket` or `github.com/gorilla/websocket` — prefer gorilla as it's already likely in the module graph via croc)
- For each WebSocket connection, open **two** TCP connections to the local croc relay:
  - Channel 0 → `127.0.0.1:9009`
  - Channel 1 → `127.0.0.1:9010`
- Bidirectionally forward bytes:
  - WebSocket → TCP: strip the channel byte, write remaining payload to the appropriate TCP connection
  - TCP → WebSocket: prepend the channel byte, write as a WebSocket binary frame
- Both channels share one WebSocket connection; use goroutines for concurrent forwarding
- When either the WebSocket or either TCP connection closes, close all three

### Pseudocode

```
accept WebSocket conn
  dial TCP to 127.0.0.1:9009 → conn0
  dial TCP to 127.0.0.1:9010 → conn1

  goroutine: ws → tcp
    loop:
      read WebSocket frame → bytes
      channel = bytes[0]
      payload = bytes[1:]
      if channel == 0: write payload → conn0
      if channel == 1: write payload → conn1

  goroutine: tcp0 → ws
    loop:
      read from conn0 → buf
      write [0x00] + buf → WebSocket

  goroutine: tcp1 → ws
    loop:
      read from conn1 → buf
      write [0x01] + buf → WebSocket

  wait for any goroutine to exit, then cancel all
```

Use a `context.Context` with cancel to coordinate shutdown across all goroutines when any one of them exits.

### Build

```
go build -o ./bin/qim-mux ./cmd/qim-mux
```

### Deployment on qimserver

Install the binary:

```bash
sudo install -m 0755 ./bin/qim-mux /srv/qim-data/bin/qim-mux
```

Create `/etc/systemd/system/qim-mux.service`:

```ini
[Unit]
Description=Qim Data WebSocket Mux
After=network-online.target qim-data-relay.service
Wants=network-online.target
Requires=qim-data-relay.service

[Service]
Type=simple
ExecStart=/srv/qim-data/bin/qim-mux --addr 127.0.0.1:9099
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
```

---

## 7. nginx Configuration (`data-relay.qim.dk.conf`)

Create `/etc/nginx/conf.d/data-relay.qim.dk.conf`. This follows the exact same WebSocket proxy pattern used in `dask.qim.dk.conf` and `jupyterhub.qim.dk.conf` on the same server.

```nginx
server {
    listen 80;
    server_name data-relay.qim.dk;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name data-relay.qim.dk;

    ssl_certificate     /etc/letsencrypt/live/data-relay.qim.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/data-relay.qim.dk/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;

    # WebSocket mux endpoint (used by qim-data tunnel fallback)
    location /relay {
        proxy_pass         http://127.0.0.1:9099;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host $host;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # Optional: health check endpoint for monitoring
    location /health {
        proxy_pass http://127.0.0.1:9099/health;
    }
}
```

**Note:** the TLS certificate for `data-relay.qim.dk` does not exist yet and must be issued before this config is activated:

```bash
sudo certbot certonly --nginx -d data-relay.qim.dk
```

The `proxy_read_timeout` and `proxy_send_timeout` must be long (3600s or more) to allow large file transfers to complete without nginx killing the WebSocket connection mid-transfer.

---

## 8. `qim-data` Client Changes (`internal/tunnel/tunnel.go`)

### When the tunnel activates

Inside `qim-data send` and `qim-data receive`, before launching the croc subprocess:

1. Attempt a TCP dial to `data-relay.qim.dk:9009` with a **3-second timeout**.
2. If it succeeds → proceed normally (direct relay connection, current behaviour).
3. If it times out → activate the WebSocket tunnel (new behaviour).

This logic lives in `internal/tunnel/tunnel.go` and is called from the send/receive command handlers.

### What the tunnel does

```
StartTunnel(relayHost string) (localPort9009 int, localPort9010 int, cancel func(), err error)
```

1. Open a WebSocket connection to `wss://data-relay.qim.dk/relay`.
2. Listen on `localhost:0` twice (OS assigns free ports) — call them `localA` and `localB`.
3. When a TCP connection arrives on `localA`:
   - All bytes received → prepend `0x00` → send over WebSocket
   - WebSocket frames with channel `0x00` → strip channel byte → write to TCP conn
4. When a TCP connection arrives on `localB`:
   - All bytes received → prepend `0x01` → send over WebSocket
   - WebSocket frames with channel `0x01` → strip channel byte → write to TCP conn
5. Return `localA`'s port as the control port (9009 substitute) and `localB`'s port as the data port (9010 substitute).
6. Return a `cancel()` function that the caller defers to clean up after croc exits.

### How croc is launched in tunnel mode

```go
localControl, localData, cancelTunnel, err := tunnel.StartTunnel("data-relay.qim.dk")
defer cancelTunnel()

// croc relay is told about the data port via environment
env := append(os.Environ(),
    fmt.Sprintf("CROC_RELAY_PORTS=9009,%d", localData),  // if croc supports this
)

// or simpler: just point relay at localControl and trust the relay to
// tell croc to use localData (which we're also proxying)
cmd := exec.Command(crocPath, "--relay", fmt.Sprintf("localhost:%d", localControl), ...)
```

**Important subtlety:** when croc connects to `localhost:localControl`, the relay tells croc "for data, connect to `localhost:9010`". But `qim-data` is listening on `localData`, not necessarily 9010. 

To handle this cleanly: **listen on fixed ports `localhost:9009` and `localhost:9010`** rather than OS-assigned ports. This matches exactly what the relay will tell croc to connect to. If those ports are in use, return an error with a clear message.

So the simpler version:

```go
func StartTunnel(relayHost string) (cancel func(), err error) {
    // listen on fixed localhost:9009 and localhost:9010
    // tunnel both over WebSocket to wss://relayHost/relay
}
```

And croc is always launched with `--relay localhost:9009`, same as the direct case.

### User-facing behaviour

The tunnel activation is **silent from the user's perspective**. No flags, no config changes. Optionally print a single line to stderr in verbose mode:

```
[qim-data] relay port 9009 unreachable, using secure tunnel via 443
```

---

## 9. `qim-data check` Integration

The existing planned `qim-data check` command should be updated to report tunnel mode:

```
✓ config present
✓ relay configured: data-relay.qim.dk:9009
✓ croc binary found: ~/.cache/qim-data/bin/croc (v10.4.2)
✗ relay TCP reachable on :9009 — port blocked
✓ relay WebSocket tunnel reachable via :443 — tunnel mode active
```

---

## 10. Sequence Diagram (Tunnel Mode)

```
qim-data (HPC)          nginx:443          qim-mux:9099    croc relay
                                                           9009  9010
    │                       │                   │            │     │
    │─── WS connect ────────►                   │            │     │
    │    /relay              │─── proxy ────────►            │     │
    │                       │                   │─── dial ──►│     │
    │                       │                   │─── dial ───────► │
    │                       │                   │            │     │
    │─── [ch=0] control ────────────────────────►──────────► │     │
    │─── [ch=1] data ────────────────────────────────────────────► │
    │                       │                   │            │     │
    │ ◄── [ch=0] response ──────────────────────◄──────────── │    │
    │ ◄── [ch=1] data ────────────────────────────────────────◄─── │
    │                       │                   │            │     │
   croc                     │                   │           croc relay
   subprocess               │                   │           (unchanged)
```

---

## 11. Implementation Order

Implement in this order to allow incremental testing:

1. **`cmd/qim-mux/main.go`** — the server mux binary, standalone, testable independently
2. **`qim-mux.service`** systemd unit + deployment steps in the runbook
3. **`data-relay.qim.dk.conf`** nginx config + certbot cert
4. **Manual integration test**: use `websocat` or a small Go test client to verify the mux works end-to-end before touching `qim-data`
5. **`internal/tunnel/tunnel.go`** — client tunnel logic
6. **Wire tunnel into `cmd/qim-data` send/receive commands**
7. **Update `qim-data check`** to report tunnel mode
8. **Update `docs/qimserver-relay-runbook.md`** with new 2-port relay config and `qim-mux` deployment steps

---

## 12. Dependencies

`qim-mux` requires a WebSocket library. Use `github.com/gorilla/websocket` — it is the standard choice and likely already present in the module graph. If not, add it:

```bash
go get github.com/gorilla/websocket
```

No other new dependencies are needed. The client tunnel uses the same library.

---

## 13. What Does NOT Change

- The `croc relay` systemd service (`qim-data-relay.service`) continues running unchanged, except for the port reduction from `9009-9013` to `9009,9010`.
- All other nginx vhosts are untouched.
- The firewall rule `9009-9013/tcp` can be narrowed to `9009-9010/tcp` but leaving it as-is causes no harm.
- Users on unrestricted networks (laptops, servers with open outbound) continue using the direct relay path with zero overhead — the tunnel is only activated when port 9009 is unreachable.
