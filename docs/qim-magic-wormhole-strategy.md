# Magic-Wormhole Server Setup on qimserver.compute.dtu.dk

## Overview

Two servers need to run, with different roles and different network exposure:

| Service | Role | Transport | Port | Exposure |
|---|---|---|---|---|
| **Mailbox server** | Key exchange / handshake only (no file data) | WebSocket | 4000 | Via nginx → `wss://wormhole-mailbox.qim.dk/v1` |
| **Transit relay** | Bulk file data fallback (when direct P2P fails) | Raw TCP | 4001 | Direct (nginx cannot proxy raw TCP on standard config) |

The transit relay carries actual file bytes and is the critical one for EU data residency. The mailbox server only ever sees a few hundred bytes of encrypted PAKE handshake — never file content.

---

## Step 1: DNS — Create Two Subdomains

In your DNS control panel for `qim.dk`, add two A records pointing to the qimserver IP:

```
wormhole-mailbox.qim.dk   A   <qimserver public IP>
wormhole-transit.qim.dk   A   <qimserver public IP>
```

To find the current IP if needed:
```bash
hostname -I
# or
ip addr show | grep 'inet ' | grep -v '127.0.0.1'
```

---

## Step 2: Create a Dedicated System User

Run as root or with sudo. All service processes will run as this user.

```bash
sudo useradd -r -s /sbin/nologin -d /var/lib/wormhole -m wormhole
sudo mkdir -p /var/lib/wormhole
sudo chown wormhole:wormhole /var/lib/wormhole
sudo chmod 750 /var/lib/wormhole
```

---

## Step 3: Install the Packages in a Python venv

Using a venv under `/opt` keeps this isolated from your conda environments and is easier to reference from systemd units.

```bash
# Verify system python3 is available (RHEL 8/9 ships python3)
python3 --version

# Create the venv
sudo python3 -m venv /opt/wormhole-venv

# Install both packages
sudo /opt/wormhole-venv/bin/pip install --upgrade pip
sudo /opt/wormhole-venv/bin/pip install \
    magic-wormhole-mailbox-server \
    magic-wormhole-transit-relay

# Verify the entry points exist
ls /opt/wormhole-venv/bin/twist
ls /opt/wormhole-venv/bin/twistd
```

### Restore SELinux file contexts on the venv

```bash
sudo restorecon -Rv /opt/wormhole-venv
```

---

## Step 4: systemd Service — Mailbox Server

Create the unit file:

```bash
sudo nano /etc/systemd/system/wormhole-mailbox.service
```

Contents:

```ini
[Unit]
Description=Magic Wormhole Mailbox Server
Documentation=https://magic-wormhole.readthedocs.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=wormhole
Group=wormhole
WorkingDirectory=/var/lib/wormhole
ExecStart=/opt/wormhole-venv/bin/twist wormhole-mailbox \
    --port=tcp:4000:interface=127.0.0.1 \
    --usage-db=/var/lib/wormhole/mailbox-usage.sqlite
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/wormhole

[Install]
WantedBy=multi-user.target
```

> **Note:** `--port=tcp:4000:interface=127.0.0.1` binds only to loopback. nginx will proxy it over TLS. The outside world never reaches port 4000 directly.

---

## Step 5: systemd Service — Transit Relay

Create the unit file:

```bash
sudo nano /etc/systemd/system/wormhole-transit.service
```

Contents:

```ini
[Unit]
Description=Magic Wormhole Transit Relay
Documentation=https://magic-wormhole.readthedocs.io
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=wormhole
Group=wormhole
WorkingDirectory=/var/lib/wormhole
ExecStart=/opt/wormhole-venv/bin/twist transitrelay \
    --port=tcp:4001 \
    --usage-db=/var/lib/wormhole/transit-usage.sqlite
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/wormhole

[Install]
WantedBy=multi-user.target
```

> **Note:** The transit relay binds on `0.0.0.0:4001` (all interfaces) because both wormhole clients connect to it directly — it cannot be proxied through nginx without the nginx stream module. Traffic is end-to-end encrypted by wormhole regardless.

---

## Step 6: SELinux Configuration

This is the most important section given SELinux is enforcing. Do these in order.

### 6a. Label port 4000 (mailbox, nginx upstream)

Port 4000 is used internally by nginx as an upstream. Label it as an HTTP port so nginx and the service can use it:

```bash
sudo semanage port -a -t http_port_t -p tcp 4000
# Verify
sudo semanage port -l | grep 4000
```

### 6b. Label port 4001 (transit relay, direct TCP)

```bash
sudo semanage port -a -t unreserved_port_t -p tcp 4001
# Verify
sudo semanage port -l | grep 4001
```

### 6c. Allow nginx to connect to local upstream (mailbox proxy)

```bash
sudo setsebool -P httpd_can_network_connect 1
# Verify
getsebool httpd_can_network_connect
```

### 6d. Allow the wormhole service to write to its data directory

The `/var/lib/wormhole` directory needs the correct context:

```bash
sudo semanage fcontext -a -t var_t "/var/lib/wormhole(/.*)?"
sudo restorecon -Rv /var/lib/wormhole
```

### 6e. Start services and watch for AVC denials

Start the services (see Step 8), then monitor for any remaining SELinux denials:

```bash
sudo ausearch -m avc -ts recent | grep wormhole
# or watch the journal
sudo journalctl -u wormhole-mailbox -u wormhole-transit -f
```

If you see AVC denials, generate and install a policy module from them:

```bash
# Capture denials for mailbox
sudo ausearch -m avc -ts recent -c twist | audit2allow -M wormhole-mailbox
sudo semodule -i wormhole-mailbox.pp

# Capture denials for transit relay
sudo ausearch -m avc -ts recent -c twist | audit2allow -M wormhole-transit
sudo semodule -i wormhole-transit.pp
```

---

## Step 7: Firewall — Open Port 4001

Port 4000 does not need a direct firewall rule (nginx handles it on 443). Port 4001 needs to be open:

```bash
sudo firewall-cmd --permanent --add-port=4001/tcp
sudo firewall-cmd --reload

# Verify
sudo firewall-cmd --list-ports
```

If you want to restrict port 4001 to DTU/university network ranges only (recommended for compliance), replace the above with a rich rule. Example for a DTU CIDR block — adjust the IP range to match:

```bash
sudo firewall-cmd --permanent --add-rich-rule='
  rule family="ipv4"
  source address="130.225.0.0/16"
  port protocol="tcp" port="4001" accept'
sudo firewall-cmd --reload
```

> Check DTU's actual IP ranges with your IT department or via `whois`.

---

## Step 8: nginx — Mailbox Server Reverse Proxy

Create the config file:

```bash
sudo nano /etc/nginx/conf.d/wormhole-mailbox.qim.dk.conf
```

Contents (HTTP-only first, for certbot to work):

```nginx
server {
    listen 80;
    server_name wormhole-mailbox.qim.dk;

    # Certbot will add SSL config here automatically
    # For now just allow the ACME challenge
    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}
```

Test and reload nginx:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### Obtain TLS certificate

```bash
sudo certbot certonly --nginx -d wormhole-mailbox.qim.dk
```

Now replace the config with the full SSL + WebSocket version:

```bash
sudo nano /etc/nginx/conf.d/wormhole-mailbox.qim.dk.conf
```

Contents:

```nginx
# Redirect HTTP → HTTPS
server {
    listen 80;
    server_name wormhole-mailbox.qim.dk;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name wormhole-mailbox.qim.dk;

    ssl_certificate     /etc/letsencrypt/live/wormhole-mailbox.qim.dk/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wormhole-mailbox.qim.dk/privkey.pem;

    # Modern TLS only
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # WebSocket proxy to the mailbox server
    location / {
        proxy_pass http://127.0.0.1:4000;
        proxy_http_version 1.1;

        # Required for WebSocket upgrade
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # Keep WebSocket connections alive for the duration of a transfer
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    # Logging
    access_log /var/log/nginx/wormhole-mailbox.access.log;
    error_log  /var/log/nginx/wormhole-mailbox.error.log;
}
```

Test and reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

---

## Step 9: Enable and Start Both Services

```bash
# Reload systemd to pick up the new unit files
sudo systemctl daemon-reload

# Enable (start on boot) and start
sudo systemctl enable --now wormhole-mailbox
sudo systemctl enable --now wormhole-transit

# Check status
sudo systemctl status wormhole-mailbox
sudo systemctl status wormhole-transit

# Tail logs
sudo journalctl -u wormhole-mailbox -f
sudo journalctl -u wormhole-transit -f
```

---

## Step 10: Verify the Services Are Listening

```bash
# Should show 127.0.0.1:4000 for mailbox (loopback only)
# Should show 0.0.0.0:4001 for transit relay
sudo ss -tlnp | grep -E '4000|4001'
```

Quick WebSocket connectivity check from any machine:

```bash
# Install wscat if needed: npm install -g wscat
wscat -c wss://wormhole-mailbox.qim.dk/v1
# Should connect (press Ctrl+C to exit)
```

Quick TCP connectivity check for transit relay:

```bash
# From an external machine (e.g. your laptop or the HPC)
nc -zv wormhole-transit.qim.dk 4001
# Should print: Connection to wormhole-transit.qim.dk 4001 port [tcp/*] succeeded!
```

---

## Step 11: Test Transfer Using Your Own Servers Exclusively

Set the environment variables on both machines so there is no chance of falling back to public servers:

```bash
export WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
export WORMHOLE_TRANSIT_HELPER=tcp:wormhole-transit.qim.dk:4001
```

### On the sender (e.g. your laptop):

```bash
export WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
export WORMHOLE_TRANSIT_HELPER=tcp:wormhole-transit.qim.dk:4001
wormhole send testfile.dat
```

### On the receiver (e.g. the HPC or the Swedish server):

```bash
export WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
# The receiver does not need --transit-helper; it is negotiated from the sender
wormhole receive <code>
```

The output should show either:
- `Sending (->tcp:wormhole-transit.qim.dk:4001)..` — transit relay used (NAT fallback)
- `Sending (<-<ip>:<port>)..` — direct P2P (relay not in data path)

In both cases the mailbox server is `wss://wormhole-mailbox.qim.dk/v1`.

### Confirm the transit relay received a connection

```bash
sudo journalctl -u wormhole-transit --since "5 minutes ago"
# You should see lines like:
# got relay connection, handshake successful
```

---

## Step 12: Make the Config Permanent for Your Users

Add to `/etc/environment` on any machine that will regularly use these servers, so the variables are set system-wide without needing to export them each session:

```bash
# /etc/environment (append these two lines)
WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
WORMHOLE_TRANSIT_HELPER=tcp:wormhole-transit.qim.dk:4001
```

Or add to `/etc/profile.d/wormhole.sh` for shell sessions only:

```bash
sudo bash -c 'cat > /etc/profile.d/wormhole.sh << EOF
export WORMHOLE_RELAY_URL=wss://wormhole-mailbox.qim.dk/v1
export WORMHOLE_TRANSIT_HELPER=tcp:wormhole-transit.qim.dk:4001
EOF'
sudo chmod 644 /etc/profile.d/wormhole.sh
```

---

## Troubleshooting

### SELinux blocking the service from binding to its port

```bash
sudo ausearch -m avc -ts today | grep wormhole
# Generate a permissive policy module from the denials
sudo ausearch -m avc -ts today | audit2allow -M wormhole-fix
sudo semodule -i wormhole-fix.pp
sudo systemctl restart wormhole-mailbox wormhole-transit
```

### nginx WebSocket proxy not upgrading

Check that your nginx has the `Upgrade` and `Connection` headers set correctly. The `proxy_read_timeout` must be high (3600s) or long-lived WebSocket connections will be dropped mid-transfer.

### Transit relay not reachable

1. Confirm the service is running: `sudo systemctl status wormhole-transit`
2. Confirm port 4001 is open: `sudo firewall-cmd --list-ports`
3. Confirm SELinux allows binding: `sudo ausearch -m avc -ts recent | grep 4001`
4. Test locally first: `nc -zv localhost 4001`

### Checking which relay is actually being used

Run with verbose output:

```bash
wormhole --relay-url=wss://wormhole-mailbox.qim.dk/v1 \
         send --transit-helper=tcp:wormhole-transit.qim.dk:4001 \
         --verbose testfile.dat
```

Look for `transit` in the output to confirm which server the file bytes flow through.

---

## Summary of What Goes Where

```
Your laptop / HPC / Swedish server
          │
          │  wss://wormhole-mailbox.qim.dk/v1   (handshake only, ~KB)
          │  ──────────────────────────────────►  nginx :443
          │                                           │
          │                                       127.0.0.1:4000
          │                                       wormhole-mailbox (systemd)
          │
          │  tcp:wormhole-transit.qim.dk:4001     (file bytes, only if no direct P2P)
          │  ──────────────────────────────────►  wormhole-transit (systemd)
          │                                       0.0.0.0:4001
          │
          │  direct TCP (when both peers routable)  ──────────────► other peer directly
          │  (transit relay not in path at all)
```

All file content remains within EU infrastructure. The public servers at `relay.magic-wormhole.io` and `transit.magic-wormhole.io` are never contacted as long as `WORMHOLE_RELAY_URL` and `WORMHOLE_TRANSIT_HELPER` are set.
