# Release Procedure

This document describes how to prepare and publish a new release of `qim-data`.

## Prerequisites

- Git configured and authorized for GitHub
- GitHub CLI (`gh`) installed and authenticated
- Go toolchain available locally
- All changes committed and pushed to `main`

## Step-by-step Release

### 1. Update version references (if needed)

Update any hardcoded version strings in code or documentation to match the release version.

### 2. Tag the release

```bash
git tag v1.0.0
git push origin v1.0.0
```

Replace `v1.0.0` with your target version.

### 3. Build binaries for all platforms

```bash
./scripts/build-releases.sh v1.0.0
```

This script:
- Builds for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64)
- Generates `SHA256SUMS` checksums in the `dist/` directory
- Compresses binaries with `-ldflags="-s -w"` for smaller size

Output example:
```
dist/qim-data_v1.0.0_linux_amd64
dist/qim-data_v1.0.0_linux_arm64
dist/qim-data_v1.0.0_macos_amd64
dist/qim-data_v1.0.0_macos_arm64
dist/qim-data_v1.0.0_windows_amd64.exe
dist/SHA256SUMS
```

### 4. Create GitHub release

**Option A: Using GitHub CLI (recommended)**

```bash
gh release create v1.0.0 dist/* --title "qim-data v1.0.0" --notes "Release notes here"
```

**Option B: Via GitHub web UI**

1. Go to: https://github.com/qim-center/qim-data/releases
2. Click "Draft a new release"
3. Select tag: `v1.0.0`
4. Title: `qim-data v1.0.0`
5. Description: Add release notes and changes
6. Drag & drop or upload all files from `dist/`
7. Click "Publish release"

### 5. Verify release

Visit: https://github.com/qim-center/qim-data/releases/tag/v1.0.0

Check that all binaries are uploaded and checksums are present.

## For Users: Downloading and Using Binaries

Users can now download pre-built binaries without needing Go installed.

### Download

1. Visit the [Releases page](https://github.com/qim-center/qim-data/releases)
2. Download the binary for their OS/arch:
   - Linux x86_64: `qim-data_vX.X.X_linux_amd64`
   - Linux ARM64: `qim-data_vX.X.X_linux_arm64`
   - macOS Intel: `qim-data_vX.X.X_macos_amd64`
   - macOS Apple Silicon: `qim-data_vX.X.X_macos_arm64`
   - Windows: `qim-data_vX.X.X_windows_amd64.exe`

### Verify checksum (optional but recommended)

```bash
# Linux/macOS
sha256sum -c SHA256SUMS

# Or check individual file
sha256sum qim-data_v1.0.0_linux_amd64
# Compare against SHA256SUMS
```

### Make executable and run (Linux/macOS)

```bash
chmod +x qim-data_v1.0.0_linux_amd64
./qim-data_v1.0.0_linux_amd64 setup
```

### Install to system path (optional)

```bash
sudo install -m 0755 qim-data_v1.0.0_linux_amd64 /usr/local/bin/qim-data
qim-data setup
```

## Future: Automated Releases with GitHub Actions

Consider adding GitHub Actions CI/CD to automate this process:
- Build on every tag push
- Generate and upload binaries
- Create draft release automatically

This is a future enhancement but not required for initial releases.
