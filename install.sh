#!/bin/sh
set -e

BINDIR="${BINDIR:-$HOME/.local/bin}"

if ! command -v go >/dev/null 2>&1; then
	echo "error: go is required to build cronit (https://go.dev/dl/)" >&2
	exit 1
fi

VERSION="$(git describe --tags --always 2>/dev/null || echo dev)"

echo "building cronit ${VERSION}..."
mkdir -p "$BINDIR"
go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$BINDIR/cronit" .

echo "installed $BINDIR/cronit"

case ":$PATH:" in
*":$BINDIR:"*) ;;
*)
	echo "note: $BINDIR is not in your PATH"
	echo "  add it, e.g.: export PATH=\"\$PATH:$BINDIR\""
	;;
esac
