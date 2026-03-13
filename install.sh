#!/bin/bash

set -e

BINARY_NAME="gitdual"
INSTALL_DIR="/usr/local/bin"
REPO_URL="https://github.com/florianjs/gitdual"

OS=$(uname -s | tr '[:upper:] ' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY_URL="${REPO_URL}/releases/latest/download/${BINARY_NAME}-${OS}-${ARCH}"

echo "Installing GitDual..."
echo "Platform: ${OS}/${ARCH}"

if command -v curl &> /dev/null; then
    curl -fsSL "${BINARY_URL}" -o "${BINARY_NAME}"
elif command -v wget &> /dev/null; then
    wget -q "${BINARY_URL}" -O "${BINARY_NAME}"
else
    echo "Error: curl or wget required"
    exit 1
fi

chmod +x "${BINARY_NAME}"

if [ -w "${INSTALL_DIR}" ]; then
    mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    echo "Sudo required for installation to ${INSTALL_DIR}"
    sudo mv "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo "✓ GitDual installed to ${INSTALL_DIR}/${BINARY_NAME}"
echo "Run 'gitdual --help' to get started"
