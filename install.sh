#!/bin/sh

set -e

REPO="mrshabel/halb"
BINARY_NAME="halb"

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        CYGWIN*|MINGW*)  echo "windows" ;;
        *)          echo "unknown" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)     echo "amd64" ;;
        amd64)      echo "amd64" ;;
        aarch64)    echo "arm64" ;;
        arm64)      echo "arm64" ;;
        *)          echo "amd64" ;;
    esac
}

install() {
    OS=$(detect_os)
    ARCH=$(detect_arch)

    if [ "$OS" = "unknown" ]; then
        echo "Unsupported operating system"
        exit 1
    fi

    VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

    if [ -z "$VERSION" ]; then
        echo "Could not fetch latest release version"
        exit 1
    fi

    echo "Installing HALB ${VERSION} for ${OS}/${ARCH}..."

    TMPDIR=$(mktemp -d)
    cd "$TMPDIR"

    if [ "$OS" = "windows" ]; then
        curl -sL "https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}_${OS}_${ARCH}.zip" -o release.zip
        unzip -o release.zip
    else
        curl -sL "https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}_${OS}_${ARCH}.tar.gz" -o release.tar.gz
        tar xzf release.tar.gz
    fi

    DEST_DIR="/usr/local/bin"
    if [ -w "$DEST_DIR" ]; then
        cp "$BINARY_NAME" "$DEST_DIR/"
        echo "Installed to ${DEST_DIR}/${BINARY_NAME}"
    else
        echo "Installing to ~/.local/bin..."
        mkdir -p "$HOME/.local/bin"
        cp "$BINARY_NAME" "$HOME/.local/bin/${BINARY_NAME}"
        echo "Add ~/.local/bin to your PATH and run: halb"
    fi

    cd /
    rm -rf "$TMPDIR"

    echo "Done! Run 'halb' to start."
}

install
