FROM golang:1.24.10-alpine AS base

RUN apk update && apk upgrade
RUN apk add --no-cache just git curl bash

WORKDIR /build

ENV GO111MODULE=auto \
    CGO_ENABLED=0 \
    GOOS=linux

COPY scripts/install.sh .
RUN chmod +x install.sh && bash ./install.sh

FROM base AS build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN just css
RUN just build linux && go build -o command cmd/command/main.go && go build -o collect_logs cmd/collect-logs/main.go

# Default final base image to Alpine Linux
FROM alpine:3.21 AS production

# Ensure we have latest packages applied
RUN apk update && apk upgrade

# Create a non-root user
RUN addgroup -g 10001 -S iota-user \
    && adduser --disabled-password --gecos '' -u 10000 --home /home/iota-user iota-user -G iota-user \
    && chown -R iota-user:iota-user /home/iota-user

WORKDIR /home/iota-user
COPY --from=build --chown=iota-user:iota-user /build/run_server ./run_server
COPY --from=build --chown=iota-user:iota-user /build/command ./command
COPY --from=build --chown=iota-user:iota-user /build/collect_logs ./collect_logs
COPY --from=build --chown=iota-user:iota-user /build/migrations ./migrations
COPY --chown=iota-user:iota-user scripts/start.sh ./start.sh

ENV PATH=/home/iota-user:$PATH

# Make startup script executable and switch to non-root user
RUN chmod +x ./start.sh
USER iota-user
CMD ["./start.sh"]
