#!/usr/bin/env bash
# Downloads the latest shellrecap release for this machine and starts it:
#
#   curl -fsSL https://raw.githubusercontent.com/ksauraj/shellrecap/master/setup.sh | bash
#
# Only uses tools that behave the same on macOS (bash 3.2, BSD utilities)
# and Linux.
set -eu

repo="ksauraj/shellrecap"
binary="shellrecap"

fail() {
  echo "Error: $*" >&2
  exit 1
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo darwin ;;
    Linux)
      if [ "$(uname -o 2>/dev/null)" = "Android" ]; then
        echo android
      else
        echo linux
      fi
      ;;
    FreeBSD) echo freebsd ;;
    OpenBSD) echo openbsd ;;
    MINGW* | MSYS* | CYGWIN*)
      fail "on Windows, download $binary-windows-amd64.exe from https://github.com/$repo/releases/latest"
      ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64)
      # A shell running under Rosetta reports x86_64 on Apple silicon
      if [ "$(uname -s)" = "Darwin" ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = "1" ]; then
        echo arm64
      else
        echo amd64
      fi
      ;;
    arm64 | aarch64) echo arm64 ;;
    armv6* | armv7* | arm) echo arm ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

os=$(detect_os)
arch=$(detect_arch)
url="https://github.com/$repo/releases/latest/download/$binary-$os-$arch"

echo "Downloading $binary for $os/$arch..." >&2
download="$binary.download"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$download" "$url" || fail "couldn't download $url"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$download" "$url" || fail "couldn't download $url"
else
  fail "curl or wget is needed to download $binary"
fi
mv "$download" "$binary"
chmod +x "$binary"

# Files downloaded through a browser are quarantined on macOS
if [ "$os" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$binary" 2>/dev/null || true
fi
echo "Downloaded ./$binary" >&2

# When this script is piped into bash, its stdin is the script itself, so
# hand the terminal to the app or it can't read the keyboard
if [ -t 0 ]; then
  exec "./$binary"
elif (exec </dev/tty) 2>/dev/null; then
  exec "./$binary" </dev/tty
else
  echo "Run ./$binary to start it." >&2
fi
