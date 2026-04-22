# qim-data Strategy (Draft v0.1)

Date: 2026-04-22

## 1. Problem and Goal

We need a reliable and user-friendly way to transfer very large datasets (up to a few TB per transfer) between research environments, starting with DTU and MAX IV.

Key constraints:

- Data transfer must remain under our infrastructure control (no public relay dependency).
- Users can use terminal commands, but they must be minimal and clear.
- It must work across major operating systems (Linux, macOS, Windows).
- We have a relay-capable host: `qimserver` (AlmaLinux 9.7) that can be exposed via subdomains like `*.qim.dk`.
- Installation and onboarding must be very easy for end users.
- End users should not need to manually handle relay passwords during normal use.

## 2. What We Can Reuse From `croc`

Based on upstream `croc` (`v10.4.2`, release dated 2026-03-11), we can leverage:

- End-to-end encryption with PAKE.
- Cross-platform CLI support.
- Interrupted transfer resume support.
- Self-hosted relay mode (`croc relay`).
- Relay customization via `--relay`, `--ports`, `--host`, and relay password via `--pass` / `CROC_PASS`.

Important technical notes from upstream behavior:

- Default relay ports are `9009,9010,9011,9012,9013` and relay requires at least 2 ports.
- Client default relay is public (`croc.schollz.com:9009` / `croc6.schollz.com:9009`), so we must override this in our tooling.
- Relay password can be loaded from a file path passed to `--pass` (useful for secrets on `qimserver`).
- Relay code paths are in-memory room coordination plus TCP piping; this is useful for compliance/operations discussions (no built-in persistent file store in relay code path).

## 3. Wrapper Around `croc`: Pros and Cons

### Pros

- Fastest path to value: we inherit a mature transfer protocol and client behavior immediately.
- Low cryptographic risk: we avoid building custom crypto or custom transport from scratch.
- Cross-platform from day one: `croc` already supports all major desktop/server OS targets.
- Better UX control: wrapper can enforce defaults (our relay, our flags) and provide short commands.
- Lower maintenance than a fork: wrapper avoids long-term protocol divergence.

### Cons

- Upstream dependency risk: flag/behavior changes in future `croc` releases can break wrapper assumptions.
- Operational limits remain `croc` limits: we do not gain enterprise features (fine-grained auth, detailed auditing, policy engine) by wrapping.
- Shared relay secret model: relay password is coarse-grained access control unless we add surrounding controls (firewall/VPN).
- Additional layer to debug: failures may happen in wrapper logic or in upstream `croc`.
- Relay remains a bandwidth chokepoint and a single point of failure.

### Verdict

The wrapper approach is the best MVP path, with explicit hardening around relay access and operations.

Additional product decision:

- We target **no user-facing relay password** in the final UX.
- We keep relay access control, but hide complexity inside `qim-data`.

## 4. Language Choice for `qim-data`

We should build the wrapper CLI in **Go**.

Why Go is best here:

- Produces single static binaries for Linux/macOS/Windows (easy onboarding).
- No runtime dependency burden (vs Python).
- Better Windows parity than shell-only approaches.
- Easy release automation with checksum/signature artifacts.

Alternatives:

- Bash: great for Linux admins, weak for Windows and long-term maintainability.
- Python: flexible, but installer/runtime management adds friction for scientific users and shared clusters.

## 5. Proposed Architecture

### 5.1 Relay Layer (`qimserver`)

Run a dedicated `croc relay` service on `qimserver` with systemd:

- DNS: `data-relay.qim.dk` -> `qimserver`.
- Open TCP ports `9009-9013` (or a custom contiguous set if needed).
- Use a strong relay password stored in a root-readable secret file (not CLI plain text).
- Restrict network exposure with firewall allowlists and/or VPN where possible.
- Monitor via systemd logs + basic host/network monitoring.

Suggested service pattern:

- Service base path: `/srv/qim-data`
- Binary path: `/srv/qim-data/bin/croc`
- Secret file: `/etc/qim-data/relay.pass` (`chmod 600`, root-owned)
- Service command:

```bash
/srv/qim-data/bin/croc --pass /etc/qim-data/relay.pass relay \
  --host 0.0.0.0 \
  --ports 9009,9010,9011,9012,9013
```

Note: Nginx reverse proxy is generally not required for `croc` relay because traffic is raw TCP on multiple ports. Direct TCP exposure plus firewalling is simpler.

### 5.2 Client Layer (`qim-data` wrapper CLI)

Provide a very small command surface:

```bash
qim-data send <file-or-folder>
qim-data receive <code>
```

Wrapper defaults:

- Always use `--relay data-relay.qim.dk:9009`.
- Do not require users to type relay passwords.
- `qim-data` injects relay credentials internally (from managed local config/secure storage).
- Keep upstream `croc` behavior for transfer semantics, resume, prompts, etc.

Initial implementation mode:

- Wrapper shells out to local `croc` binary (clear errors if missing).
- Wrapper installs or updates a pinned `croc` version automatically per platform.
- User-facing install should be one command (Linux/macOS) or one package manager command (Windows).

### 5.3 Authentication UX (Final Direction)

Goal: easy for users, controlled for operations.

- Users run `qim-data send ...` / `qim-data receive ...` with no manual relay secret handling.
- `qim-data` stores relay credentials once during onboarding (`qim-data setup`).
- Credentials are reused silently by the wrapper when invoking `croc`.

Operational note:

- True "no relay password at all" is only acceptable if network access is tightly restricted (VPN/IP allowlist/private routing), otherwise relay abuse risk is high.

## 6. Security and Compliance Baseline

- Do not use `croc` default relay password (`pass123`) in production.
- Rotate relay password on a schedule and after staff changes/incidents.
- Prefer network-level controls (VPN, IP allowlist to DTU/MAX IV ranges).
- Keep relay host hardened and patched (AlmaLinux + security updates).
- Keep `croc` version pinned and reviewed; update intentionally after compatibility checks.

## 7. Performance Expectations for TB-Scale Transfers

- Relay will carry both ingress and egress traffic, so capacity planning must include full pass-through load.
- Multi-TB transfers depend mostly on WAN quality and relay NIC/CPU/network path.
- Start with upstream default ports/transfers to reduce complexity, then tune only if measurements require it.
- Validate resume behavior under real link interruptions as a required acceptance test.

## 8. Delivery Plan (Phased)

### Phase 1: Relay MVP (Infrastructure)

- Bring up `croc relay` on `qimserver` with systemd.
- Configure DNS + firewall.
- Run internal sender/receiver test through `data-relay.qim.dk`.

Exit criteria:

- Successful transfer of test files across two independent hosts.
- Relay survives restart (`systemctl restart`) without manual intervention.

### Phase 2: `qim-data` CLI MVP (User Experience)

- Build Go wrapper with `send` and `receive`.
- Enforce relay defaults.
- Add `qim-data doctor` for environment checks (`croc` present, relay reachable, config valid).
- Add `qim-data setup` to store relay credential once, so users do not export `CROC_PASS`.

Exit criteria:

- New user can transfer data with two commands after one-time setup.

### Phase 3: Distribution and Onboarding

- Release signed binaries for Linux/macOS/Windows.
- Provide a one-command install path per OS.
- Add concise quickstart docs for DTU/MAX IV users.
- Add minimal troubleshooting guide (ports, auth, resume, firewall).

Exit criteria:

- Installation + first transfer in under 10 minutes on each major OS.

### Phase 4: Hardening and Operations

- Add structured logging conventions and runbook.
- Add password rotation procedure.
- Add reliability tests with large synthetic data.

Exit criteria:

- Reproducible operational procedure and incident response checklist.

## 9. Known Risks and Mitigations

- Single relay SPOF.
  Mitigation: backups/snapshots, rebuild runbook, optional secondary relay in next iteration.

- Shared relay secret leakage.
  Mitigation: rotate secret, distribute securely, enforce network restrictions, consider identity-aware access in future.

- Upstream incompatibility drift.
  Mitigation: pin tested `croc` version(s), add compatibility CI for wrapper against target versions.

- User confusion around Linux/macOS secure mode (`CROC_SECRET` behavior).
  Mitigation: wrapper hides this complexity and keeps commands consistent.

## 10. Immediate Next Step

After approval of this strategy, implementation starts with:

1. `qimserver` systemd + firewall setup instructions (operator runbook) under `/srv/qim-data`.
2. Go CLI skeleton (`qim-data send`, `qim-data receive`, `qim-data doctor`, `qim-data setup`) with relay defaults and hidden credential handling.
3. End-to-end integration test between two hosts using `data-relay.qim.dk`.

## 11. Progress Notes

### 2026-04-22

- Relay service is running on `qimserver` via systemd.
- SELinux execution labeling for `/srv/qim-data/bin/croc` is documented and validated.
- Firewall and DNS were validated for external TCP access on `9009-9013`.
- Real transfer test succeeded (`2.8 GB` scale, approx `120 MB/s`) using `data-relay.qim.dk`.
- Wrapper implementation started in this repo (`qim-data` Go CLI scaffold + docs).
- `qim-data setup` implementation now auto-installs pinned `croc` builds for major user platforms.
