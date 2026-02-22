#!/bin/sh
# install.sh - Universal installer for cloudterminal
# Usage:
#   curl -sSL https://raw.githubusercontent.com/tmyktmyc/BlinkingLine-cloudterminal/main/install.sh | sh
#   curl -sSL ... | sh -s -- --version v0.2.0 --dir /usr/local/bin
#
# Options:
#   --version VERSION   Install a specific version (e.g. v0.2.0). Default: latest.
#   --dir DIRECTORY     Install to this directory. Default: ~/.local/bin
#   --help              Show this help message.

set -eu

REPO="tmyktmyc/BlinkingLine-cloudterminal"
BINARY="cloudterminal"
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
INSTALL_DIR=""
VERSION=""

# --- Helpers ---

log() {
    printf '%s\n' "$1"
}

err() {
    printf 'Error: %s\n' "$1" >&2
    exit 1
}

usage() {
    log "Usage: install.sh [--version VERSION] [--dir DIRECTORY] [--help]"
    log ""
    log "Options:"
    log "  --version VERSION   Install a specific version (e.g. v0.2.0)"
    log "  --dir DIRECTORY     Install to this directory (default: ~/.local/bin)"
    log "  --help              Show this help message"
    exit 0
}

# --- HTTP fetch (supports curl and wget) ---

has_cmd() {
    command -v "$1" >/dev/null 2>&1
}

http_get() {
    url="$1"
    if has_cmd curl; then
        curl -fsSL "$url"
    elif has_cmd wget; then
        wget -qO- "$url"
    else
        err "Neither curl nor wget found. Please install one of them."
    fi
}

http_download() {
    url="$1"
    dest="$2"
    if has_cmd curl; then
        curl -fsSL -o "$dest" "$url"
    elif has_cmd wget; then
        wget -qO "$dest" "$url"
    else
        err "Neither curl nor wget found. Please install one of them."
    fi
}

# --- OS / architecture detection ---

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)   echo "linux" ;;
        Darwin*)  echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*)  echo "windows" ;;
        *)        err "Unsupported operating system: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              err "Unsupported architecture: $arch" ;;
    esac
}

# --- Version resolution ---

get_latest_version() {
    # GitHub API returns JSON; extract tag_name without jq dependency.
    response="$(http_get "https://api.github.com/repos/${REPO}/releases/latest")"
    # Parse tag_name from JSON using sed
    tag="$(printf '%s' "$response" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
    if [ -z "$tag" ]; then
        err "Could not determine latest version from GitHub API. Try specifying --version."
    fi
    echo "$tag"
}

# --- Main ---

main() {
    # Parse arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            --version)
                [ $# -ge 2 ] || err "--version requires an argument"
                VERSION="$2"
                shift 2
                ;;
            --dir)
                [ $# -ge 2 ] || err "--dir requires an argument"
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help|-h)
                usage
                ;;
            *)
                err "Unknown option: $1. Use --help for usage."
                ;;
        esac
    done

    # Defaults
    if [ -z "$INSTALL_DIR" ]; then
        INSTALL_DIR="$DEFAULT_INSTALL_DIR"
    fi

    log "Installing ${BINARY}..."

    # Detect platform
    os="$(detect_os)"
    arch="$(detect_arch)"
    log "  Platform: ${os}/${arch}"

    # Windows arm64 is not supported by GoReleaser config
    if [ "$os" = "windows" ] && [ "$arch" = "arm64" ]; then
        err "Windows arm64 is not supported. Use amd64 instead."
    fi

    # Resolve version
    if [ -z "$VERSION" ]; then
        log "  Fetching latest version..."
        VERSION="$(get_latest_version)"
    fi
    log "  Version: ${VERSION}"

    # Strip leading 'v' for the archive filename (GoReleaser uses version without v prefix)
    version_number="${VERSION#v}"

    # Determine archive format and binary name
    if [ "$os" = "windows" ]; then
        ext="zip"
        binary_name="${BINARY}.exe"
    else
        ext="tar.gz"
        binary_name="${BINARY}"
    fi

    archive="${BINARY}_${version_number}_${os}_${arch}.${ext}"
    download_url="https://github.com/${REPO}/releases/download/${VERSION}/${archive}"

    # Create temp directory with cleanup trap
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT INT TERM

    # Download
    log "  Downloading ${download_url}..."
    http_download "$download_url" "${tmpdir}/${archive}"

    # Verify checksum
    checksums_url="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
    log "  Verifying checksum..."
    if http_download "$checksums_url" "${tmpdir}/checksums.txt"; then
        expected_checksum="$(grep "  ${archive}\$" "${tmpdir}/checksums.txt" | cut -d ' ' -f 1)"
        if [ -z "$expected_checksum" ]; then
            # Also try single-space separator (BSD style)
            expected_checksum="$(grep " ${archive}\$" "${tmpdir}/checksums.txt" | cut -d ' ' -f 1)"
        fi
        if [ -z "$expected_checksum" ]; then
            err "Could not find checksum for ${archive} in checksums.txt"
        fi
        if has_cmd sha256sum; then
            actual_checksum="$(sha256sum "${tmpdir}/${archive}" | cut -d ' ' -f 1)"
        elif has_cmd shasum; then
            actual_checksum="$(shasum -a 256 "${tmpdir}/${archive}" | cut -d ' ' -f 1)"
        else
            log "  WARNING: Neither sha256sum nor shasum found; skipping checksum verification."
            actual_checksum=""
        fi
        if [ -n "$actual_checksum" ]; then
            if [ "$actual_checksum" != "$expected_checksum" ]; then
                err "Checksum mismatch! Expected ${expected_checksum}, got ${actual_checksum}."
            fi
            log "  Checksum verified."
        fi
    else
        log "  WARNING: Could not download checksums.txt; skipping verification."
    fi

    # Extract
    log "  Extracting..."
    if [ "$ext" = "zip" ]; then
        if has_cmd unzip; then
            unzip -qo "${tmpdir}/${archive}" -d "${tmpdir}"
        else
            err "unzip is required to extract Windows archives."
        fi
    else
        tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}"
    fi

    # Verify binary was extracted
    if [ ! -f "${tmpdir}/${binary_name}" ]; then
        # Some GoReleaser setups nest in a directory; search for it
        extracted="$(find "$tmpdir" -name "$binary_name" -type f 2>/dev/null | head -1)"
        if [ -z "$extracted" ]; then
            err "Could not find ${binary_name} binary in the archive."
        fi
        mv "$extracted" "${tmpdir}/${binary_name}"
    fi

    # macOS: remove quarantine attribute (silent fail on Linux)
    xattr -d com.apple.quarantine "${tmpdir}/${binary_name}" 2>/dev/null || true

    # Create install directory if needed
    mkdir -p "$INSTALL_DIR"

    # Install
    mv "${tmpdir}/${binary_name}" "${INSTALL_DIR}/${binary_name}"
    chmod +x "${INSTALL_DIR}/${binary_name}"
    log "  Installed to ${INSTALL_DIR}/${binary_name}"

    # Check if install directory is in PATH
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*)
            ;;
        *)
            log ""
            log "  WARNING: ${INSTALL_DIR} is not in your PATH."
            log "  Add it by running:"
            log ""
            log "    export PATH=\"${INSTALL_DIR}:\$PATH\""
            log ""
            log "  To make it permanent, add the line above to your shell profile"
            log "  (~/.bashrc, ~/.zshrc, or ~/.profile)."
            ;;
    esac

    # Verify installation
    if [ -x "${INSTALL_DIR}/${binary_name}" ]; then
        installed_version="$("${INSTALL_DIR}/${binary_name}" --version 2>/dev/null || echo "unknown")"
        log ""
        log "  ${BINARY} ${installed_version} installed successfully!"
    else
        err "Installation verification failed."
    fi
}

main "$@"
