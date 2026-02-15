FROM mcr.microsoft.com/playwright:v1.48.2-jammy

ARG GO_VERSION=1.23.2
ARG ARCH=amd64
ARG PNPM_VERSION=9.15.0
ARG TEMPL_VERSION=v0.3.857
ARG GOLANGCI_LINT_VERSION=v2.1.6

ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:/go/bin:${PATH}
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      git \
      jq \
      make \
      pgformatter \
      postgresql-client \
      unzip && \
    rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tgz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf /tmp/go.tgz && \
    rm /tmp/go.tgz

RUN npm install -g "pnpm@${PNPM_VERSION}"

RUN set -euo pipefail; \
    curl --proto '=https' --tlsv1.2 -fsSL https://just.systems/install.sh -o /tmp/just-install.sh && \
    bash /tmp/just-install.sh --to /usr/local/bin && \
    rm -f /tmp/just-install.sh

RUN go install "github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}"

RUN set -euo pipefail; \
    curl -fsSL "https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCI_LINT_VERSION}/install.sh" -o /tmp/golangci-lint-install.sh && \
    sh /tmp/golangci-lint-install.sh -b /usr/local/bin "${GOLANGCI_LINT_VERSION}" && \
    rm -f /tmp/golangci-lint-install.sh

RUN go version && \
    node --version && \
    pnpm --version && \
    just --version && \
    golangci-lint version && \
    templ --help >/dev/null
