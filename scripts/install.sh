#!/usr/bin/env sh
#
# Structured Vibe installer.
#
#   curl -fsSL https://raw.githubusercontent.com/mwenkdev/structured-vibe/main/scripts/install.sh | sh
#
# Installs the svibe CLI and its managed runtime payload: the core pack, the
# model registry, and the OpenCode integration. These belong to one release
# version and are installed together (architecture 17.1).
#
# A checksum mismatch is a hard failure. There is deliberately no override
# flag: an artifact that does not match its published checksum is not the
# artifact we published.
#
# Environment:
#   SVIBE_VERSION      version to install (default: latest release)
#   SVIBE_INSTALL_DIR  directory for the binary (default: $HOME/.local/bin)
#   SVIBE_CONFIG_HOME  managed payload root (default: OS config dir + /svibe)

set -eu

REPO="mwenkdev/structured-vibe"
GITHUB="https://github.com"
API="https://api.github.com"

log()  { printf '%s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# --- platform detection ----------------------------------------------------

detect_os() {
	os=$(uname -s)
	case "$os" in
		Linux)  echo linux ;;
		Darwin) echo darwin ;;
		MINGW*|MSYS*|CYGWIN*)
			die "detected Windows under $os; download the windows archive from $GITHUB/$REPO/releases and extract it manually" ;;
		*) die "unsupported operating system: $os" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
		x86_64|amd64)  echo amd64 ;;
		arm64|aarch64) echo arm64 ;;
		*) die "unsupported architecture: $arch" ;;
	esac
}

# config_root mirrors Go's os.UserConfigDir, which svibe uses to find its
# managed payload. These must agree or the CLI will not find what we install.
config_root() {
	if [ -n "${SVIBE_CONFIG_HOME:-}" ]; then
		printf '%s\n' "$SVIBE_CONFIG_HOME"
		return
	fi
	case "$1" in
		darwin) printf '%s\n' "$HOME/Library/Application Support/svibe" ;;
		*)      printf '%s\n' "${XDG_CONFIG_HOME:-$HOME/.config}/svibe" ;;
	esac
}

# --- release resolution ----------------------------------------------------

latest_version() {
	# Release names and tags use plain SemVer with no "v" prefix.
	curl -fsSL "$API/repos/$REPO/releases/latest" \
		| sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
		| head -n 1
}

# --- checksum verification -------------------------------------------------

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		die "no sha256 tool found (need sha256sum or shasum)"
	fi
}

verify_checksum() {
	archive_path=$1
	archive_name=$2
	checksums=$3

	expected=$(awk -v name="$archive_name" '$2 == name || $2 == "*" name {print $1}' "$checksums" | head -n 1)
	[ -n "$expected" ] || die "no published checksum for $archive_name"

	actual=$(sha256_of "$archive_path")
	if [ "$expected" != "$actual" ]; then
		die "checksum mismatch for $archive_name
  expected $expected
  actual   $actual
This artifact does not match the published checksum. Installation aborted."
	fi
	log "  checksum verified"
}

# --- install ---------------------------------------------------------------

main() {
	need curl
	need tar
	need uname
	need awk

	os=$(detect_os)
	arch=$(detect_arch)

	version=${SVIBE_VERSION:-}
	if [ -z "$version" ]; then
		version=$(latest_version) || true
		[ -n "$version" ] || die "cannot determine the latest release; set SVIBE_VERSION"
	fi

	name="svibe_${version}_${os}_${arch}"
	archive="${name}.tar.gz"
	base="$GITHUB/$REPO/releases/download/$version"

	install_dir=${SVIBE_INSTALL_DIR:-$HOME/.local/bin}
	cfg=$(config_root "$os")

	log "Installing svibe $version ($os/$arch)"

	tmp=$(mktemp -d)
	# shellcheck disable=SC2064
	trap "rm -rf '$tmp'" EXIT INT TERM

	log "  downloading $archive"
	curl -fsSL "$base/$archive" -o "$tmp/$archive" \
		|| die "cannot download $base/$archive"
	curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" \
		|| die "cannot download published checksums"

	verify_checksum "$tmp/$archive" "$archive" "$tmp/checksums.txt"

	tar -xzf "$tmp/$archive" -C "$tmp" || die "cannot extract $archive"
	src="$tmp/$name"
	[ -d "$src" ] || die "unexpected archive layout: $name/ not found"

	# Binary.
	mkdir -p "$install_dir" || die "cannot create $install_dir"
	cp "$src/svibe" "$install_dir/svibe" || die "cannot install the binary"
	chmod 0755 "$install_dir/svibe"

	# Managed payload. An upgrade replaces managed files unconditionally and
	# does not merge, preserve or back up local modifications (architecture
	# 16.7). Only managed directories are touched: user packs and generated
	# output are left alone.
	mkdir -p "$cfg" || die "cannot create $cfg"
	rm -rf "$cfg/core" "$cfg/config" "$cfg/integrations"
	cp -R "$src/core" "$cfg/core"
	mkdir -p "$cfg/config" "$cfg/integrations"
	cp "$src/config/models.yaml" "$cfg/config/models.yaml"
	cp -R "$src/integrations/opencode" "$cfg/integrations/opencode"

	log "  installed $install_dir/svibe"
	log "  managed payload in $cfg"
	log ""

	case ":$PATH:" in
		*":$install_dir:"*) ;;
		*)
			warn "$install_dir is not on your PATH"
			log "  add it to your shell profile:"
			log "    export PATH=\"$install_dir:\$PATH\""
			log ""
			;;
	esac

	log "Next steps:"
	log "  svibe admin setup opencode   install the OpenCode integration"
	log "  svibe init                   set up the current repository"
	log "  svibe sync                   publish skills to the host"
}

main "$@"
