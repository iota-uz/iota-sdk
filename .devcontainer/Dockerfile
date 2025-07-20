FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm

# Install TailwindCSS
RUN curl -sSfL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.15/tailwindcss-linux-x64 -o /usr/local/bin/tailwindcss \
    && chmod +x /usr/local/bin/tailwindcss

# Install Go tools
RUN go install github.com/a-h/templ/cmd/templ@v0.3.857 \
    && go install github.com/air-verse/air@v1.61.5 \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 \
    && go install golang.org/x/tools/cmd/goimports@v0.31.0 \
    && go install github.com/loov/goda@v0.5.11 \
    && go install github.com/iota-uz/devhub/cmd/devhub@v0.0.2

# Install additional tools
RUN apt-get update && apt-get install -y \
    make \
    postgresql-client \
    redis-tools \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Set up Go environment
ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOPATH=/go \
    AIR_SILENT=true

# Create directories for Air
RUN mkdir -p /tmp/air

WORKDIR /workspace