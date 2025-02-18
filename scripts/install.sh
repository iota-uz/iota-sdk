#!/bin/bash

case "$(uname -m)" in
    "x86_64") ARCH="x64" ;;
    "aarch64") ARCH="arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)" && exit 1 ;;
esac

curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.15/tailwindcss-linux-${ARCH}
chmod +x tailwindcss-linux-${ARCH}
mv tailwindcss-linux-${ARCH} /usr/local/bin/tailwindcss
echo "tailwindcss installed successfully"
