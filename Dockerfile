# Install Stage
FROM golang:1.23.2 AS install-stage

WORKDIR /build

RUN case "$(uname -m)" in \
      "x86_64") ARCH="x64" ;; \
      "aarch64") ARCH="arm64" ;; \
      *) echo "Unsupported architecture: $(uname -m)" && exit 1 ;; \
    esac && \
    curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.15/tailwindcss-linux-${ARCH} && \
    chmod +x tailwindcss-linux-${ARCH} && \
    mv tailwindcss-linux-${ARCH} /usr/local/bin/tailwindcss

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
RUN go install github.com/a-h/templ/cmd/templ@v0.2.793 && go install github.com/mitranim/gow@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make generate && go vet ./...
RUN make css
RUN make release

# Production Stage
FROM alpine:3.21 AS production

RUN apk update && apk upgrade

RUN addgroup -g 10001 -S iota-user \
    && adduser --disabled-password --gecos '' -u 10000 --home /home/iota-user iota-user -G iota-user \
    && chown -R iota-user:iota-user /home/iota-user

WORKDIR /home/iota-user
COPY --from=install-stage /build/run_server ./run_server

ENV PATH=/home/iota-user:$PATH

USER iota-user
ENTRYPOINT run_server

# Staging Stage
FROM install-stage AS staging
RUN go build -o run_server cmd/server/main.go && go build -o seed_db cmd/seed/main.go
CMD go run cmd/migrate/main.go up && /build/seed_db && /build/run_server

# Testing CI Stage
FROM install-stage AS testing-ci
CMD [ "go", "test", "-v", "./..." ]

# Testing Local Stage
FROM install-stage AS testing-local
CMD [ "gow", "test", "./..." ]

# Development Stage
FROM golang:1.23.2-alpine AS dev

WORKDIR /app

# Install required system dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    git \
    curl \
    make

# Install Air for hot reloading
RUN curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.5

# Install TailwindCSS
RUN case "$(uname -m)" in \
      "x86_64") ARCH="x64" ;; \
      "aarch64") ARCH="arm64" ;; \
      *) echo "Unsupported architecture: $(uname -m)" && exit 1 ;; \
    esac && \
    curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/download/v3.4.15/tailwindcss-linux-${ARCH} && \
    chmod +x tailwindcss-linux-${ARCH} && \
    mv tailwindcss-linux-${ARCH} /usr/local/bin/tailwindcss

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@v0.2.793

# Copy Go modules and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Set Air to run on container startup
ENTRYPOINT ["air"]
