# Qimserver Relay Runbook (`qim-data`)

Date: 2026-04-22
Target host: `qimserver` (AlmaLinux 9.7)
Relay DNS: `data-relay.qim.dk`
Service root: `/srv/qim-data`

## 1. Prepare directories

```bash
sudo mkdir -p /srv/qim-data/bin
sudo mkdir -p /etc/qim-data
```

## 2. Install `croc` binary on the server

Choose one of the two methods below.

Method A (recommended): copy a vetted binary to `qimserver` and place it at `/srv/qim-data/bin/croc`.

Method B: build from source directly on `qimserver`.

```bash
sudo dnf -y install git golang
cd /tmp
git clone https://github.com/schollz/croc.git
cd croc
go build -ldflags="-s -w" -o croc .
sudo install -m 0755 croc /srv/qim-data/bin/croc
```

Verify:

```bash
/srv/qim-data/bin/croc --version
```

## 2.1 SELinux labeling for `/srv/qim-data/bin/croc` (required on enforcing hosts)

If SELinux is enforcing, label the custom binary path so systemd can execute it:

```bash
sudo dnf -y install policycoreutils-python-utils
# Add mapping (or modify it if it already exists)
sudo semanage fcontext -a -t bin_t '/srv/qim-data/bin(/.*)?' || \
sudo semanage fcontext -m -t bin_t '/srv/qim-data/bin(/.*)?'
sudo restorecon -Rv /srv/qim-data/bin
ls -lZ /srv/qim-data/bin/croc
```

Expected context type on the binary should include `:bin_t:`.

If you still see `:var_t:` on `/srv/qim-data/bin/croc`, verify the local SELinux rule and reapply:

```bash
sudo semanage fcontext -l | grep -E '/srv/qim-data/bin\(/\.\*\)\?'
sudo restorecon -RFvv /srv/qim-data/bin/croc
ls -lZ /srv/qim-data/bin/croc
```

## 3. Create relay secret

Generate and store a strong relay password:

```bash
openssl rand -base64 48 | sudo tee /etc/qim-data/relay.pass >/dev/null
sudo chmod 600 /etc/qim-data/relay.pass
sudo chown root:root /etc/qim-data/relay.pass
```

## 4. Create systemd service

Create `/etc/systemd/system/qim-data-relay.service`:

```ini
[Unit]
Description=Qim Data Relay (croc)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/srv/qim-data/bin/croc --pass /etc/qim-data/relay.pass relay --host 0.0.0.0 --ports 9009,9010,9011,9012,9013
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadOnlyPaths=/srv/qim-data/bin/croc
ReadOnlyPaths=/etc/qim-data/relay.pass
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now qim-data-relay.service
sudo systemctl status qim-data-relay.service --no-pager
```

Follow logs:

```bash
journalctl -u qim-data-relay.service -f
```

Important: `--pass` is a global `croc` flag, so it must be placed before `relay`.

## 5. Open firewall ports (AlmaLinux firewalld)

Check which zone your external interface is using (example: `public`):

```bash
sudo firewall-cmd --get-active-zones
```

Open relay ports in that zone (here we use `public`), then reload:

```bash
sudo firewall-cmd --zone=public --permanent --add-port=9009-9013/tcp
sudo firewall-cmd --reload
sudo firewall-cmd --zone=public --list-ports
```

## 6. DNS setup

Create/verify DNS A/AAAA record:

- `data-relay.qim.dk` -> public IP of `qimserver`

Validate from a client machine:

```bash
dig +short data-relay.qim.dk
```

## 7. End-to-end validation (two-host test)

Precondition: both test hosts have `croc` installed and know the relay password.

Version note: use the same `croc` major version on sender and receiver (pin `v10.x`).

Set relay password on both hosts:

```bash
export CROC_PASS='PASTE_THE_SECRET_FROM_/etc/qim-data/relay.pass'
```

Sender host:

```bash
croc --relay data-relay.qim.dk:9009 send /path/to/testfile.bin
```

Receiver host:

```bash
croc --relay data-relay.qim.dk:9009
```

Then paste the receive code when prompted.

Non-interactive alternative:

```bash
CROC_SECRET='<CODE_FROM_SENDER>' croc --relay data-relay.qim.dk:9009
```

Recommended staged tests:

1. 100 MB file
2. 5-10 GB file
3. interrupted transfer/resume test

## 7.1 Client version pinning (recommended)

Check on each client:

```bash
croc --version
```

If version is below `v10`, replace it with pinned release `v10.4.2` (Linux amd64 example):

```bash
cd /tmp
curl -fL -o croc.tar.gz https://github.com/schollz/croc/releases/download/v10.4.2/croc_v10.4.2_Linux-64bit.tar.gz
tar -xzf croc.tar.gz
sudo install -m 0755 croc /usr/local/bin/croc
/usr/local/bin/croc --version
```

## 8. Operational checks

Service health:

```bash
systemctl is-active qim-data-relay.service
```

Port listening check:

```bash
sudo ss -lntp | grep -E ':9009|:9010|:9011|:9012|:9013'
```

Restart drill:

```bash
sudo systemctl restart qim-data-relay.service
sudo systemctl status qim-data-relay.service --no-pager
```

## 9. Password rotation procedure

```bash
openssl rand -base64 48 | sudo tee /etc/qim-data/relay.pass.new >/dev/null
sudo chmod 600 /etc/qim-data/relay.pass.new
sudo chown root:root /etc/qim-data/relay.pass.new
sudo mv /etc/qim-data/relay.pass.new /etc/qim-data/relay.pass
sudo systemctl restart qim-data-relay.service
```

After rotation:

1. Distribute the new password through approved secure channels.
2. Re-test one transfer.
3. Invalidate any old stored credentials in automation/scripts.

## 10. Troubleshooting quick list

If clients cannot connect:

1. Check DNS resolution for `data-relay.qim.dk`.
2. Confirm TCP ports `9009-9013` are open and reachable.
3. Check service logs: `journalctl -u qim-data-relay.service -n 200 --no-pager`.
4. Verify relay password match between client and server.
5. On Linux/macOS, if using non-interactive receive, ensure `CROC_SECRET` is set.

If service fails to start:

1. Validate binary: `/srv/qim-data/bin/croc --version`.
2. Validate secret permissions: `ls -l /etc/qim-data/relay.pass`.
3. Check unit syntax: `sudo systemd-analyze verify /etc/systemd/system/qim-data-relay.service`.
4. Check SELinux denials: `sudo ausearch -m avc -ts recent | tail -n 50`.
5. Explain SELinux denial reason: `sudo ausearch -m avc -ts recent | audit2why`.
6. Confirm binary label: `ls -lZ /srv/qim-data/bin/croc` (should contain `:bin_t:`).
7. Verify relay starts manually with correct flag order:

```bash
sudo /srv/qim-data/bin/croc --pass /etc/qim-data/relay.pass relay --host 0.0.0.0 --ports 9009,9010,9011,9012,9013
```
