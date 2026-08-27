#!/bin/sh
# Downloads and installs a prebuilt `ten` binary from GitHub Releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rinsyan0518/ten/main/install.sh | sh
#
# Env vars:
#   TEN_VERSION  Release tag to install, e.g. "v0.1.0" (default: latest)
#   INSTALL_DIR  Directory to install the binary into (default: $HOME/.local/bin)
set -eu

REPO="rinsyan0518/ten"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${TEN_VERSION:-latest}"

log() {
	echo "install: $*"
}

fail() {
	echo "install: $*" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

need_cmd curl
need_cmd tar

detect_os() {
	case "$(uname -s)" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) fail "unsupported OS: $(uname -s) (ten supports Linux and macOS only)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	*) fail "unsupported architecture: $(uname -m) (ten supports amd64 and arm64 only)" ;;
	esac
}

sha256_check() {
	# file checksums_file
	file="$1"
	checksums_file="$2"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "$(dirname "$file")" && grep " $(basename "$file")\$" "$checksums_file" | sha256sum -c -) >/dev/null
	elif command -v shasum >/dev/null 2>&1; then
		(cd "$(dirname "$file")" && grep " $(basename "$file")\$" "$checksums_file" | shasum -a 256 -c -) >/dev/null
	else
		fail "required command not found: sha256sum or shasum"
	fi
}

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$VERSION" = "latest" ]; then
	log "resolving latest release"
	VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
	[ -n "$VERSION" ] || fail "could not resolve latest release version"
fi

VERSION_NUM="${VERSION#v}"
ARCHIVE="ten_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

log "downloading ${ARCHIVE} (${VERSION})"
curl -fsSL -o "${TMP_DIR}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" || fail "failed to download ${BASE_URL}/${ARCHIVE}"
curl -fsSL -o "${TMP_DIR}/checksums.txt" "${BASE_URL}/checksums.txt" || fail "failed to download checksums.txt"

log "verifying checksum"
sha256_check "${TMP_DIR}/${ARCHIVE}" "${TMP_DIR}/checksums.txt" || fail "checksum verification failed for ${ARCHIVE}"

log "extracting"
tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}" ten

mkdir -p "$INSTALL_DIR"
mv "${TMP_DIR}/ten" "${INSTALL_DIR}/ten"
chmod 755 "${INSTALL_DIR}/ten"

log "installed ten ${VERSION} to ${INSTALL_DIR}/ten"

case ":$PATH:" in
*":${INSTALL_DIR}:"*) ;;
*) log "warning: ${INSTALL_DIR} is not in your \$PATH; add it to use 'ten' directly" ;;
esac
