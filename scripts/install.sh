#!/usr/bin/env bash
# install.sh — install the golinks CLI from source
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY="golinks"

if ! command -v go &>/dev/null; then
    echo "Error: Go is required to build golinks." >&2
    echo "Install Go from https://go.dev/dl/ then re-run this script." >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Building golinks CLI..."
CGO_ENABLED=0 go build -ldflags "-s -w" -o "/tmp/${BINARY}" "${REPO_ROOT}/cmd/cli"

echo "Installing to ${INSTALL_DIR}/${BINARY} ..."
if [[ ! -w "$INSTALL_DIR" ]]; then
    sudo mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY}"
else
    mv "/tmp/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"
fi

echo "Done! Verify with: golinks --help"
echo ""
echo "Quick start:"
echo "  golinks config set server http://localhost:8080"
echo "  golinks ls"
