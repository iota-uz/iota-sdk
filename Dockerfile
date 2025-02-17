FROM golang:1.23.2-alpine AS base

RUN apk update && apk upgrade
RUN apk add --no-cache make git curl bash

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
RUN make css
RUN make release && go build -o migrate cmd/migrate/main.go && go build -o seed_db cmd/seed/main.go

# Default final base image to Alpine Linux
FROM alpine:3.21 AS production

# Ensure we have latest packages applied
RUN apk update && apk upgrade

# Create a non-root user
RUN addgroup -g 10001 -S iota-user \
    && adduser --disabled-password --gecos '' -u 10000 --home /home/iota-user iota-user -G iota-user \
    && chown -R iota-user:iota-user /home/iota-user

WORKDIR /home/iota-user
COPY --from=build /build/run_server ./run_server
COPY --from=build /build/migrate ./migrate
COPY --from=build /build/seed_db ./seed_db

ENV PATH=/home/iota-user:$PATH

USER iota-user
CMD ["/bin/sh", "-c", "migrate redo && seed_db && run_server"]

