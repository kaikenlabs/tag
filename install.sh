#!/bin/sh
# TAG installer script
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh -s -- --version v0.2.0
#   curl -sSfL https://raw.githubusercontent.com/kaikenlabs/tag/main/install.sh | sh -s -- --dir /usr/local/bin
#
# Options:
#   --version VERSION   Install a specific version (default: latest)
#   --dir DIR           Installation directory (default: ./bin)
#   --no-verify         Skip checksum verification
#
# Environment:
#   GITHUB_TOKEN        Optional token for GitHub API (avoids rate limits)

set -e

OWNER="kaikenlabs"
REPO="tag"
BINARY="tag"
GITHUB_API="https://api.github.com"
GITHUB_RELEASES="https://github.com/${OWNER}/${REPO}/releases"

# Defaults
INSTALL_DIR="./bin"
VERSION=""
VERIFY_CHECKSUM=1

usage() {
    cat <<EOF
TAG installer

Usage:
    curl -sSfL <url>/install.sh | sh -s -- [options]

Options:
    --version VERSION   Install a specific version (default: latest)
    --dir DIR           Installation directory (default: ./bin)
    --no-verify         Skip checksum verification
    -h, --help          Show this help message
EOF
}

log() {
    printf "[tag] %s\n" "$*"
}

err() {
    printf "[tag] ERROR: %s\n" "$*" >&2
    exit 1
}

# Parse arguments
while [ $# -gt 0 ]; do
    case "$1" in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --no-verify)
            VERIFY_CHECKSUM=0
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            err "unknown option: $1"
            ;;
    esac
done

detect_os() {
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "Linux" ;;
        Darwin*) echo "Darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "Windows" ;;
        *) err "unsupported operating system: $os" ;;
    esac
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)  echo "x86_64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) err "unsupported architecture: $arch" ;;
    esac
}

has_cmd() {
    command -v "$1" >/dev/null 2>&1
}

http_get() {
    url="$1"
    if has_cmd curl; then
        if [ -n "$GITHUB_TOKEN" ]; then
            curl -sSfL -H "Authorization: token ${GITHUB_TOKEN}" "$url"
        else
            curl -sSfL "$url"
        fi
    elif has_cmd wget; then
        if [ -n "$GITHUB_TOKEN" ]; then
            wget -qO- --header="Authorization: token ${GITHUB_TOKEN}" "$url"
        else
            wget -qO- "$url"
        fi
    else
        err "either curl or wget is required"
    fi
}

http_download() {
    url="$1"
    dest="$2"
    if has_cmd curl; then
        if [ -n "$GITHUB_TOKEN" ]; then
            curl -sSfL -H "Authorization: token ${GITHUB_TOKEN}" -o "$dest" "$url"
        else
            curl -sSfL -o "$dest" "$url"
        fi
    elif has_cmd wget; then
        if [ -n "$GITHUB_TOKEN" ]; then
            wget -q --header="Authorization: token ${GITHUB_TOKEN}" -O "$dest" "$url"
        else
            wget -q -O "$dest" "$url"
        fi
    else
        err "either curl or wget is required"
    fi
}

get_latest_version() {
    # Use the GitHub API to get the latest release tag
    response="$(http_get "${GITHUB_API}/repos/${OWNER}/${REPO}/releases/latest")"
    version="$(printf '%s' "$response" | grep '"tag_name"' | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')"
    if [ -z "$version" ]; then
        err "could not determine latest version (GitHub API rate limit?). Try setting GITHUB_TOKEN or use --version"
    fi
    echo "$version"
}

find_checksum_tool() {
    if has_cmd sha256sum; then
        echo "sha256sum"
    elif has_cmd gsha256sum; then
        echo "gsha256sum"
    elif has_cmd shasum; then
        echo "shasum"
    elif has_cmd openssl; then
        echo "openssl"
    else
        echo ""
    fi
}

verify_checksum() {
    archive="$1"
    checksums_file="$2"
    expected_file="$(basename "$archive")"

    tool="$(find_checksum_tool)"
    if [ -z "$tool" ]; then
        log "WARNING: no checksum tool found, skipping verification"
        return 0
    fi

    expected="$(grep "${expected_file}" "$checksums_file" | awk '{print $1}')"
    if [ -z "$expected" ]; then
        err "checksum not found for ${expected_file} in checksums file"
    fi

    case "$tool" in
        sha256sum|gsha256sum)
            actual="$("$tool" "$archive" | awk '{print $1}')"
            ;;
        shasum)
            actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
            ;;
        openssl)
            actual="$(openssl dgst -sha256 "$archive" | awk '{print $NF}')"
            ;;
    esac

    if [ "$expected" != "$actual" ]; then
        err "checksum mismatch for ${expected_file}
  expected: ${expected}
  actual:   ${actual}"
    fi

    log "checksum verified"
}

install() {
    os="$(detect_os)"
    arch="$(detect_arch)"

    # Reject unsupported combinations
    if [ "$os" = "Windows" ] && [ "$arch" = "arm64" ]; then
        err "windows/arm64 is not supported"
    fi

    # Determine version
    if [ -z "$VERSION" ]; then
        log "finding latest version..."
        VERSION="$(get_latest_version)"
    fi
    # Ensure version starts with 'v'
    case "$VERSION" in
        v*) ;;
        *)  VERSION="v${VERSION}" ;;
    esac
    # Strip the 'v' prefix for the archive filename
    version_num="${VERSION#v}"

    log "installing tag ${VERSION} for ${os}/${arch}"

    # Determine archive extension
    if [ "$os" = "Windows" ]; then
        ext="zip"
    else
        ext="tar.gz"
    fi

    archive_name="${BINARY}_${version_num}_${os}_${arch}.${ext}"
    download_url="${GITHUB_RELEASES}/download/${VERSION}/${archive_name}"
    checksums_url="${GITHUB_RELEASES}/download/${VERSION}/checksums.txt"

    # Create temp directory
    tmpdir="$(mktemp -d)"
    trap 'rm -rf "$tmpdir"' EXIT

    # Download archive
    log "downloading ${download_url}"
    http_download "$download_url" "${tmpdir}/${archive_name}"

    # Verify checksum
    if [ "$VERIFY_CHECKSUM" = "1" ]; then
        log "downloading checksums..."
        http_download "$checksums_url" "${tmpdir}/checksums.txt"
        verify_checksum "${tmpdir}/${archive_name}" "${tmpdir}/checksums.txt"
    fi

    # Extract
    log "extracting..."
    case "$ext" in
        tar.gz)
            tar xzf "${tmpdir}/${archive_name}" -C "$tmpdir"
            ;;
        zip)
            if has_cmd unzip; then
                unzip -q "${tmpdir}/${archive_name}" -d "$tmpdir"
            else
                err "unzip is required to extract .zip archives"
            fi
            ;;
    esac

    # Install binary
    mkdir -p "$INSTALL_DIR"
    if [ "$os" = "Windows" ]; then
        cp "${tmpdir}/${BINARY}.exe" "${INSTALL_DIR}/${BINARY}.exe"
    else
        cp "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
        chmod +x "${INSTALL_DIR}/${BINARY}"
    fi

    installed_path="$(cd "$INSTALL_DIR" && pwd)/${BINARY}"
    log "installed ${VERSION} to ${installed_path}"

    # Check if install dir is in PATH
    case ":${PATH}:" in
        *":$(cd "$INSTALL_DIR" && pwd):"*) ;;
        *)
            log ""
            log "NOTE: ${INSTALL_DIR} is not in your PATH. Add it with:"
            log "  export PATH=\"$(cd "$INSTALL_DIR" && pwd):\$PATH\""
            ;;
    esac
}

install
