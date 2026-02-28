#!/usr/bin/env bash
set -euo pipefail

JUST_VERSION="${JUST_VERSION:-1.46.0}"
JUST_SHA256="${JUST_SHA256:-}"

if [[ -z "${JUST_SHA256}" ]]; then
  echo "Environment variable JUST_SHA256 is required."
  exit 1
fi

if command -v just >/dev/null; then
  installed_version=$(just --version 2>/dev/null | awk '{print $2}')
  if [[ -n "${installed_version}" ]] && \
    [[ "${JUST_VERSION}" = "${installed_version}" || "$(printf "%s\n%s\n" "${JUST_VERSION}" "${installed_version}" | sort -V | head -n1)" = "${JUST_VERSION}" ]]; then
    just --version
    exit 0
  fi
fi

echo "Downloading and installing just ${JUST_VERSION}."
tarball="/tmp/just-${JUST_VERSION}.tar.gz"

if ! curl -fsSL --retry 2 --retry-connrefused --retry-delay 5 \
  -o "${tarball}" "https://github.com/casey/just/releases/download/${JUST_VERSION}/just-${JUST_VERSION}-x86_64-unknown-linux-musl.tar.gz"; then
  echo "Failed to download just ${JUST_VERSION}."
  rm -f "${tarball}"
  exit 1
fi

echo "${JUST_SHA256}  ${tarball}" | sha256sum -c -

sudo tar -xzf "${tarball}" -C /tmp
sudo install -m 755 /tmp/just /usr/local/bin/just

rm -f "${tarball}"
just --version
