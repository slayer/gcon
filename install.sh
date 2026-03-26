#!/bin/sh
# gcon installer — downloads the latest (or specified) release binary.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/slayer/gcon/master/install.sh | sh
#   curl -sSL https://raw.githubusercontent.com/slayer/gcon/master/install.sh | sh -s -- v0.7.0

set -e

REPO="slayer/gcon"
INSTALL_DIR="/usr/local/bin"
VERSION="${1:-}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)
    echo "Error: unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Resolve version — fetch latest tag if not specified
if [ -z "$VERSION" ]; then
  VERSION="$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
  if [ -z "$VERSION" ]; then
    echo "Error: could not determine latest version" >&2
    exit 1
  fi
fi

# Normalize — ensure leading 'v' for tag, strip for archive name
case "$VERSION" in
  v*) VERSION_NUM="${VERSION#v}" ;;
  *)  VERSION_NUM="$VERSION"; VERSION="v${VERSION}" ;;
esac

ARCHIVE="gcon_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading gcon ${VERSION} for ${OS}/${ARCH}..."
curl -sSfL -o "${TMPDIR}/${ARCHIVE}" "$URL"
curl -sSfL -o "${TMPDIR}/checksums.txt" "$CHECKSUM_URL"

# Verify checksum
echo "Verifying checksum..."
EXPECTED="$(grep "${ARCHIVE}" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
if [ -z "$EXPECTED" ]; then
  echo "Error: checksum not found for ${ARCHIVE}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
elif command -v openssl >/dev/null 2>&1; then
  ACTUAL="$(openssl dgst -sha256 "${TMPDIR}/${ARCHIVE}" | awk '{print $2}')"
else
  echo "Error: no SHA-256 checksum tool found (sha256sum, shasum, or openssl)" >&2
  exit 1
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "Error: checksum mismatch" >&2
  echo "  expected: ${EXPECTED}" >&2
  echo "  got:      ${ACTUAL}" >&2
  exit 1
fi

# Extract and locate binary (handles both flat and wrapped archives)
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

GCON_BIN="${TMPDIR}/gcon"
if [ ! -f "$GCON_BIN" ]; then
  for d in "${TMPDIR}"/gcon_*; do
    [ -d "$d" ] || continue
    if [ -f "$d/gcon" ]; then
      GCON_BIN="$d/gcon"
      break
    fi
  done
fi

if [ ! -f "$GCON_BIN" ]; then
  echo "Error: could not find extracted gcon binary in ${TMPDIR}" >&2
  exit 1
fi

# Install — fall back to ~/.local/bin if no write access to /usr/local/bin
if [ -w "$INSTALL_DIR" ] || [ -w "$(dirname "$INSTALL_DIR")" ]; then
  mkdir -p "$INSTALL_DIR"
  mv "$GCON_BIN" "${INSTALL_DIR}/gcon"
  echo "Installed gcon to ${INSTALL_DIR}/gcon"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  mv "$GCON_BIN" "${INSTALL_DIR}/gcon"
  echo "Installed gcon to ${INSTALL_DIR}/gcon"
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "Note: add ${INSTALL_DIR} to your PATH" ;;
  esac
fi

# Confirm
"${INSTALL_DIR}/gcon" --version
