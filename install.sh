#!/bin/bash

REPO="ghousemohamed/simple-tunnel"
BIN_NAME="simple-tunnel"
INSTALL_DIR="$HOME/.local/bin"
VERSION="v0.0.1"

mkdir -p "$INSTALL_DIR"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [[ "$OS" == "linux" ]]; then
    if [[ "$ARCH" == "x86_64" ]]; then
        BINARY="$BIN_NAME-linux-amd64"
    elif [[ "$ARCH" == "aarch64" ]]; then
        BINARY="$BIN_NAME-linux-arm64"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
elif [[ "$OS" == "darwin" ]]; then
    if [[ "$ARCH" == "x86_64" ]]; then
        BINARY="$BIN_NAME-darwin-amd64"
    elif [[ "$ARCH" == "arm64" ]]; then
        BINARY="$BIN_NAME-darwin-arm64"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
elif [[ "$OS" == "mingw32nt" || "$OS" == "cygwin" || "$OS" == "msys" ]]; then
    BINARY="$BIN_NAME-windows-amd64.exe"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY"
echo "Downloading $BINARY from $DOWNLOAD_URL..."
curl -L -o "$INSTALL_DIR/$BIN_NAME" "$DOWNLOAD_URL"

if [[ "$OS" != "mingw32nt" && "$OS" != "cygwin" && "$OS" != "msys" ]]; then
    chmod +x "$INSTALL_DIR/$BIN_NAME"
fi

if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo "Adding $INSTALL_DIR to PATH"
    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$HOME/.bashrc"
    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$HOME/.bash_profile"
    echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$HOME/.profile"
    echo "Please restart your terminal or run 'source ~/.bashrc' to update your PATH."
fi

echo "Installation complete! You can now run '$BIN_NAME' from anywhere."