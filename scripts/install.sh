#!/bin/sh
# Usage: curl -fsSL https://raw.githubusercontent.com/awf-project/cli/main/scripts/install.sh | sh
set -eu

REPO="awf-project/cli"
BINARY="awf"
INSTALL_DIR="/usr/local/bin"

# --- Helpers ----------------------------------------------------------------

log() { printf '%s\n' "$@"; }
err() { log "Error: $*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "'$1' is required but not found"
}

# --- Detect OS/Arch ---------------------------------------------------------

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *)       err "unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)             err "unsupported architecture: $(uname -m)" ;;
  esac
}

detect_ext() {
  case "$1" in
    darwin) echo "zip" ;;
    *)      echo "tar.gz" ;;
  esac
}

# --- Fetch latest version ---------------------------------------------------

latest_version() {
  need curl
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
}

# --- Verify checksum --------------------------------------------------------

verify_checksum() {
  _vc_archive="$1"
  _vc_checksums="$2"

  need sha256sum

  _vc_expected=$(grep "$(basename "$_vc_archive")" "$_vc_checksums" | awk '{print $1}')
  _vc_actual=$(sha256sum "$_vc_archive" | awk '{print $1}')

  if [ "$_vc_expected" != "$_vc_actual" ]; then
    err "checksum mismatch for $(basename "$_vc_archive"): expected $_vc_expected, got $_vc_actual"
  fi

  log "Checksum verified."
}

# --- Main -------------------------------------------------------------------

main() {
  need curl

  OS=$(detect_os)
  ARCH=$(detect_arch)
  EXT=$(detect_ext "$OS")
  VERSION="${AWF_VERSION:-$(latest_version)}"

  log "Installing ${BINARY} ${VERSION} (${OS}/${ARCH})..."

  ARCHIVE="${BINARY}_${OS}_${ARCH}.${EXT}"
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  log "Downloading ${ARCHIVE}..."
  curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}"

  log "Downloading checksums..."
  curl -fsSL -o "${TMPDIR}/checksums.txt" "${BASE_URL}/checksums.txt"

  verify_checksum "${TMPDIR}/${ARCHIVE}" "${TMPDIR}/checksums.txt"

  log "Extracting..."
  case "$EXT" in
    tar.gz) tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR" ;;
    zip)    need unzip; unzip -qo "${TMPDIR}/${ARCHIVE}" -d "$TMPDIR" ;;
  esac

  log "Installing to ${INSTALL_DIR}/${BINARY}..."
  if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  else
    sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  fi
  chmod +x "${INSTALL_DIR}/${BINARY}"

  log ""
  log "${BINARY} ${VERSION} installed successfully."
  log "Run '${BINARY} --help' to get started."
}

main
