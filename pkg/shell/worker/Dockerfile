# shell-worker — the isolated runtime behind the SDK shell capability.
#
# The build context is this directory: the worker is a self-contained Go module
# (Go 1.26, gbash pinned in go.mod/go.sum) and does not need the SDK sources.
#
# Build:
#   docker build -t iotauz/shell-worker:dev pkg/shell/worker
# Run (see README for the full deployment policy):
#   docker run --rm --read-only --memory 512m --cap-drop ALL \
#     --security-opt no-new-privileges -e SHELL_WORKER_TOKEN=... -p 8080:8080 \
#     iotauz/shell-worker:dev
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shell-worker ./cmd/shell-worker

# Static, shell-less, non-root base: the guest never runs host binaries, and
# the image has none to offer.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/shell-worker /shell-worker
USER nonroot:nonroot
ENV SHELL_WORKER_ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/shell-worker"]
