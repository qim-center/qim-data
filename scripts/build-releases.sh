#!/usr/bin/env bash
set -e

# Build qim-data binaries for multiple platforms
# Usage: ./scripts/build-releases.sh [VERSION]
#
# Example:
#   ./scripts/build-releases.sh v1.0.0
#   ./scripts/build-releases.sh v1.0.0-beta

VERSION="${1:-dev}"
BUILD_DIR="dist"
LDFLAGS="-s -w -X main.Version=${VERSION}"

# Target platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

echo "Building qim-data ${VERSION}..."
mkdir -p "${BUILD_DIR}"

# Clean old builds
rm -f "${BUILD_DIR}"/qim-data_*

for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  
  # Friendly names
  os_display="${os}"
  case "${os}" in
    darwin) os_display="macos" ;;
  esac
  
  output="${BUILD_DIR}/qim-data_${VERSION}_${os_display}_${arch}"
  [ "${os}" = "windows" ] && output="${output}.exe"
  
  echo "  Building ${target}..."
  GOOS="${os}" GOARCH="${arch}" go build \
    -ldflags="${LDFLAGS}" \
    -o "${output}" \
    ./cmd/qim-data
done

# Generate checksums
echo ""
echo "Generating checksums..."
cd "${BUILD_DIR}"
shasum -a 256 qim-data_* > SHA256SUMS
cat SHA256SUMS
cd ..

echo ""
echo "✓ Build complete!"
echo ""
echo "Artifacts in ${BUILD_DIR}/"
ls -lh "${BUILD_DIR}"/qim-data_*
echo ""
echo "Next: Create a GitHub release and upload these files."
echo "  See: https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository"
