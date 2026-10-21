#!/bin/sh
# Installs skelly from a GitHub release, after checking the download against the release's
# checksums.txt. Run it with --help for the options.
#
# SKELLY_BASE_URL replaces https://github.com/aaron-vaz/skelly/releases, for a mirror. It must be
# https, except for a localhost URL, which the tests use.
#
# Nothing is installed unless the checksum matches, and no step needs root: the default
# directory is the user's own. The whole script is inside main, which runs on the last line, so
# a download that is cut short runs nothing.

set -eu

REPO="aaron-vaz/skelly"
DEFAULT_BASE_URL="https://github.com/${REPO}/releases"

tmpdir=""

say() {
    printf '%s\n' "$*"
}

die() {
    printf 'install.sh: error: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    if [ -n "$tmpdir" ]; then
        rm -rf "$tmpdir"
    fi
}

usage() {
    cat <<'USAGE'
Installs skelly from a GitHub release, after checking it against the release's checksums.txt.

  curl -fsSL https://raw.githubusercontent.com/aaron-vaz/skelly/main/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/aaron-vaz/skelly/main/install.sh | sh -s -- -v v0.6.0 -b /usr/local/bin

Options (each can also be set in the environment):
  -v, --version VERSION    a release tag such as v0.6.0 (SKELLY_VERSION), the latest release by default
  -b, --bin-dir DIR        where to put skelly (SKELLY_INSTALL_DIR), $HOME/.local/bin by default
  -p, --verify-provenance  also check the build attestation with the GitHub CLI (gh), which must be installed
  -h, --help               show this text
USAGE
}

need() {
    command -v "$1" >/dev/null 2>&1 || die "$1 is required but was not found"
}

# A release tag, and nothing that could be read as a path or as shell syntax.
valid_version() {
    printf '%s' "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$'
}

detect_os() {
    case "$(uname -s)" in
        Linux) echo linux ;;
        Darwin) echo darwin ;;
        *) die "unsupported operating system $(uname -s): skelly is released for Linux and macOS" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64 | amd64) echo amd64 ;;
        arm64 | aarch64) echo arm64 ;;
        *) die "unsupported architecture $(uname -m): skelly is released for amd64 and arm64" ;;
    esac
}

# Only https, so the download cannot be read or changed on the way, except from this machine.
curl_proto() {
    case "$1" in
        https://*) echo "=https" ;;
        http://localhost | http://localhost[:/]* | http://127.0.0.1 | http://127.0.0.1[:/]*) echo "=http,https" ;;
        *) die "SKELLY_BASE_URL must be an https URL, got $1" ;;
    esac
}

fetch() {
    # $1 URL, $2 output file
    curl --proto "$proto" --tlsv1.2 --fail --silent --show-error --location --retry 2 \
        --output "$2" "$1" || die "could not download $1"
}

latest_version() {
    # The latest release redirects to its tag page, which names the tag without using the API
    final=$(curl --proto "$proto" --tlsv1.2 --fail --silent --show-error --location --head \
        --output /dev/null --write-out '%{url_effective}' "$base_url/latest") ||
        die "could not find the latest release at $base_url/latest"
    echo "${final##*/}"
}

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d ' ' -f 1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | cut -d ' ' -f 1
    else
        die "sha256sum or shasum is required to check the download"
    fi
}

verify_checksum() {
    # $1 asset name, $2 directory holding the asset and checksums.txt
    asset="$1"
    dir="$2"

    expected=$(awk -v name="$asset" '{ f = $2; sub(/^\*/, "", f); if (f == name) print tolower($1) }' "$dir/checksums.txt")
    [ -n "$expected" ] || die "checksums.txt has no entry for $asset: this release cannot be verified, so nothing was installed"
    [ "$(printf '%s\n' "$expected" | wc -l | tr -d ' ')" -eq 1 ] ||
        die "checksums.txt has more than one entry for $asset, so nothing was installed"
    printf '%s' "$expected" | grep -Eq '^[0-9a-f]{64}$' || die "the checksum for $asset in checksums.txt is not a SHA-256 hash"

    actual=$(sha256_of "$dir/$asset")
    if [ "$actual" != "$expected" ]; then
        die "the checksum of $asset does not match checksums.txt (expected $expected, got $actual): nothing was installed"
    fi
}

verify_provenance() {
    # $1 path of the downloaded binary
    command -v gh >/dev/null 2>&1 || die "--verify-provenance needs the GitHub CLI (gh), which was not found"
    gh attestation verify "$1" --repo "$REPO" >/dev/null ||
        die "the build attestation of the download could not be verified, so nothing was installed"
    say "Build attestation verified."
}

install_binary() {
    # $1 downloaded file, $2 destination directory
    mkdir -p "$2" || die "could not create $2: pick another directory with -b, or run with the rights to write to it"
    staged="$2/.skelly.$$"
    cp "$1" "$staged" || die "could not write to $2: pick another directory with -b, or run with the rights to write to it"
    chmod 0755 "$staged"
    # A move within the directory, so an interrupted install never leaves half a binary at the final name
    mv -f "$staged" "$2/skelly" || {
        rm -f "$staged"
        die "could not replace $2/skelly"
    }
}

main() {
    version="${SKELLY_VERSION:-}"
    bin_dir="${SKELLY_INSTALL_DIR:-}"
    provenance=0

    while [ $# -gt 0 ]; do
        case "$1" in
            -v | --version)
                [ $# -ge 2 ] || die "$1 needs a value"
                version="$2"
                shift 2
                ;;
            -b | --bin-dir)
                [ $# -ge 2 ] || die "$1 needs a value"
                bin_dir="$2"
                shift 2
                ;;
            -p | --verify-provenance)
                provenance=1
                shift
                ;;
            -h | --help)
                usage
                exit 0
                ;;
            *) die "unknown option $1 (see --help)" ;;
        esac
    done

    if [ -z "$bin_dir" ]; then
        [ -n "${HOME:-}" ] || die "HOME is not set: choose where to install with -b"
        bin_dir="$HOME/.local/bin"
    fi

    if [ -n "$version" ]; then
        case "$version" in
            v*) ;;
            *) version="v$version" ;;
        esac
        valid_version "$version" || die "$version is not a release version such as v0.6.0"
    fi

    base_url="${SKELLY_BASE_URL:-$DEFAULT_BASE_URL}"
    base_url="${base_url%/}"
    proto=$(curl_proto "$base_url")

    for tool in curl uname mktemp awk grep wc tr cut; do
        need "$tool"
    done
    os=$(detect_os)
    arch=$(detect_arch)

    if [ -z "$version" ]; then
        version=$(latest_version)
        valid_version "$version" || die "the latest release is called $version, which is not a release version"
    fi

    asset="skelly_${os}_${arch}"
    tmpdir=$(mktemp -d) || die "could not create a temporary directory"
    trap cleanup EXIT
    trap 'exit 1' INT TERM HUP

    say "Installing skelly $version for $os/$arch"
    fetch "$base_url/download/$version/$asset" "$tmpdir/$asset"
    fetch "$base_url/download/$version/checksums.txt" "$tmpdir/checksums.txt"

    verify_checksum "$asset" "$tmpdir"
    say "Checksum verified."
    if [ "$provenance" -eq 1 ]; then
        verify_provenance "$tmpdir/$asset"
    fi

    install_binary "$tmpdir/$asset" "$bin_dir"
    say "Installed $bin_dir/skelly"
    "$bin_dir/skelly" version || true

    case ":${PATH:-}:" in
        *":$bin_dir:"*) ;;
        *) say "Note: $bin_dir is not in your PATH." ;;
    esac
}

main "$@"
