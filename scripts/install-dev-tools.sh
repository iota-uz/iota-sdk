#!/bin/bash

set -e

# Cleanup on error
trap 'echo -e "\n${RED}Installation failed. Please check the error messages above.${NC}"; exit 1' ERR

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper function to download with retry
download_file() {
    local url=$1
    local output=$2
    local retries=3
    local count=0

    while [ $count -lt $retries ]; do
        if curl -fSL "$url" -o "$output" 2>/dev/null; then
            return 0
        fi
        count=$((count + 1))
        if [ $count -lt $retries ]; then
            echo -n "retry..."
        fi
    done

    echo -e "${RED}Failed to download from $url${NC}"
    return 1
}

# Tool versions (from go.mod and installation.md)
TEMPL_VERSION="v0.3.857"
AIR_VERSION="v1.61.5"
TAILWINDCSS_VERSION="v3.4.13"
GOLANGCI_LINT_VERSION="v1.64.8"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}IOTA SDK - Development Tools Installer${NC}"
echo -e "${GREEN}========================================${NC}\n"

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)

if [[ "$OS" != "Darwin" && "$OS" != "Linux" ]]; then
    echo -e "${RED}Error: This script only supports macOS and Linux${NC}"
    exit 1
fi

# Set platform-specific variables
if [[ "$OS" == "Darwin" ]]; then
    # macOS
    if [[ "$ARCH" == "arm64" ]]; then
        TAILWIND_ARCH="macos-arm64"
        BIN_DIR="/opt/homebrew/bin"
    else
        TAILWIND_ARCH="macos-x64"
        BIN_DIR="/usr/local/bin"
    fi
    PACKAGE_MANAGER="brew"
else
    # Linux
    if [[ "$ARCH" == "x86_64" ]]; then
        TAILWIND_ARCH="linux-x64"
    elif [[ "$ARCH" == "aarch64" || "$ARCH" == "arm64" ]]; then
        TAILWIND_ARCH="linux-arm64"
    else
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
    fi
    BIN_DIR="/usr/local/bin"

    # Detect Linux package manager
    if command -v apt-get &> /dev/null; then
        PACKAGE_MANAGER="apt"
    elif command -v yum &> /dev/null; then
        PACKAGE_MANAGER="yum"
    elif command -v pacman &> /dev/null; then
        PACKAGE_MANAGER="pacman"
    else
        PACKAGE_MANAGER="none"
    fi
fi

echo -e "${YELLOW}Detected OS: $OS${NC}"
echo -e "${YELLOW}Detected architecture: $ARCH${NC}"
echo -e "${YELLOW}Binary directory: $BIN_DIR${NC}\n"

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check network connectivity
echo -n "Checking network connectivity... "
if curl -fsSL --connect-timeout 5 https://github.com > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Failed${NC}"
    echo -e "${RED}Error: Cannot reach GitHub. Please check your internet connection.${NC}"
    exit 1
fi

# Check Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go from https://golang.org/doc/install"
    exit 1
fi
echo -e "${GREEN}✓ Go $(go version | awk '{print $3}') found${NC}"

# Check package manager (Homebrew for macOS)
if [[ "$OS" == "Darwin" ]]; then
    if ! command -v brew &> /dev/null; then
        echo -e "${RED}Error: Homebrew is not installed${NC}"
        echo "Install Homebrew from https://brew.sh"
        exit 1
    fi
    echo -e "${GREEN}✓ Homebrew found${NC}"
fi
echo ""

# Ensure GOPATH/bin exists
GOPATH=$(go env GOPATH)
mkdir -p "$GOPATH/bin"

echo -e "${YELLOW}Installing Go development tools...${NC}"

# Install templ
echo -n "Installing templ $TEMPL_VERSION... "
go install github.com/a-h/templ/cmd/templ@$TEMPL_VERSION 2>&1 > /dev/null
echo -e "${GREEN}✓${NC}"

# Install air
echo -n "Installing air $AIR_VERSION... "
go install github.com/air-verse/air@$AIR_VERSION 2>&1 > /dev/null
echo -e "${GREEN}✓${NC}"

# Install goimports
echo -n "Installing goimports (latest)... "
go install golang.org/x/tools/cmd/goimports@latest 2>&1 > /dev/null
echo -e "${GREEN}✓${NC}"

# Install golangci-lint
echo -n "Installing golangci-lint $GOLANGCI_LINT_VERSION... "
if curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$GOPATH/bin" $GOLANGCI_LINT_VERSION > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Failed${NC}"
    exit 1
fi

# Install TailwindCSS standalone
echo -n "Installing TailwindCSS $TAILWINDCSS_VERSION... "
TAILWIND_URL="https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWINDCSS_VERSION}/tailwindcss-${TAILWIND_ARCH}"
if download_file "$TAILWIND_URL" "$GOPATH/bin/tailwindcss"; then
    chmod +x "$GOPATH/bin/tailwindcss"
    # Validate the binary
    if [[ ! -x "$GOPATH/bin/tailwindcss" ]] || [[ ! -s "$GOPATH/bin/tailwindcss" ]]; then
        echo -e "${RED}✗ Downloaded binary is invalid${NC}"
        rm -f "$GOPATH/bin/tailwindcss"
        exit 1
    fi
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Failed${NC}"
    exit 1
fi
echo ""

echo -e "${YELLOW}Installing additional packages...${NC}"

# Install cloudflared
echo -n "Installing cloudflared... "
if ! command -v cloudflared &> /dev/null; then
    if [[ "$OS" == "Darwin" ]]; then
        if brew install cloudflare/cloudflare/cloudflared > /dev/null 2>&1; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${RED}✗ Failed${NC}"
        fi
    else
        # Linux - download binary
        CLOUDFLARED_ARCH="${ARCH/x86_64/amd64}"
        CLOUDFLARED_ARCH="${CLOUDFLARED_ARCH/aarch64/arm64}"
        CLOUDFLARED_URL="https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CLOUDFLARED_ARCH}"

        if sudo -n true 2>/dev/null; then
            TEMP_FILE="/tmp/cloudflared-$$"
            if download_file "$CLOUDFLARED_URL" "$TEMP_FILE"; then
                sudo mv "$TEMP_FILE" /usr/local/bin/cloudflared
                sudo chmod +x /usr/local/bin/cloudflared
                echo -e "${GREEN}✓${NC}"
            else
                echo -e "${RED}✗ Failed${NC}"
                rm -f "$TEMP_FILE"
            fi
        else
            echo -e "${YELLOW}⚠ Skipped (requires sudo)${NC}"
        fi
    fi
else
    echo -e "${GREEN}✓ (already installed)${NC}"
fi

echo ""
echo -e "${YELLOW}Creating symlinks in $BIN_DIR...${NC}"

# Create symlinks for all tools
TOOLS=("templ" "air" "goimports" "golangci-lint" "tailwindcss")

# Check if we need sudo for creating symlinks
NEEDS_SUDO=false
if [[ "$OS" == "Linux" ]] || [[ ! -w "$BIN_DIR" ]]; then
    NEEDS_SUDO=true
fi

for tool in "${TOOLS[@]}"; do
    echo -n "Symlinking $tool... "
    if [[ "$NEEDS_SUDO" == true ]]; then
        if sudo -n true 2>/dev/null; then
            sudo ln -sf "$GOPATH/bin/$tool" "$BIN_DIR/$tool"
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${YELLOW}⚠ Skipped (requires sudo)${NC}"
        fi
    else
        ln -sf "$GOPATH/bin/$tool" "$BIN_DIR/$tool"
        echo -e "${GREEN}✓${NC}"
    fi
done

echo ""
echo -e "${YELLOW}Setting up PATH in ~/.zshenv...${NC}"

# Add GOPATH/bin to PATH in .zshenv if not already present
if ! grep -q 'export PATH="$HOME/go/bin:$PATH"' ~/.zshenv 2>/dev/null; then
    echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshenv
    echo -e "${GREEN}✓ Added GOPATH/bin to PATH${NC}"
else
    echo -e "${GREEN}✓ PATH already configured${NC}"
fi

echo ""
echo -e "${YELLOW}Verifying installations...${NC}"

# Verify all tools
verify_tool() {
    local tool=$1
    local version_cmd=$2

    if command -v "$tool" &> /dev/null; then
        local version=$($version_cmd 2>&1 | head -1)
        echo -e "${GREEN}✓ $tool${NC} - $version"
        return 0
    else
        echo -e "${RED}✗ $tool - Not found in PATH${NC}"
        return 1
    fi
}

verify_tool "go" "go version"
verify_tool "templ" "templ version"
verify_tool "air" "air -v 2>&1 | tail -1"
verify_tool "goimports" "echo 'installed'"
verify_tool "golangci-lint" "golangci-lint version 2>&1 | head -1"
verify_tool "tailwindcss" "echo 'installed'"
verify_tool "cloudflared" "cloudflared --version 2>&1"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Installation complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Restart your terminal or run: source ~/.zshenv"
echo "2. Run: just deps (to install Go dependencies)"
echo "3. Run: just db local (to start PostgreSQL)"
echo "4. Run: just db migrate up && just db seed (to set up database)"
echo "5. Run: air (to start development server)"
echo ""
echo -e "${YELLOW}For more information, see docs/getting-started/installation.md${NC}"
