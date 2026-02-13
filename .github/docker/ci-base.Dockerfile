FROM mcr.microsoft.com/playwright:v1.48.2-jammy

ARG GO_VERSION=1.23.2
ARG PNPM_VERSION=9.15.0
ARG TEMPL_VERSION=v0.3.857
ARG GOLANGCI_LINT_VERSION=v2.1.6

ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:/go/bin:${PATH}

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

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf /tmp/go.tgz && \
    rm /tmp/go.tgz

RUN npm install -g "pnpm@${PNPM_VERSION}"

RUN curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin

RUN go install "github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}"

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin "${GOLANGCI_LINT_VERSION}"

RUN go version && \
    node --version && \
    pnpm --version && \
    just --version && \
    golangci-lint version && \
    templ --help >/dev/null
