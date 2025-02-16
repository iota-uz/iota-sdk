FROM golang:1.23.2 AS base

WORKDIR /build

COPY scripts/install.sh .
RUN chmod +x install.sh && ./install.sh && go install github.com/a-h/templ/cmd/templ@v0.3.819

FROM base AS build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make generate && go vet ./...
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
CMD ["/bin/sh", "-c", "./migrate && ./seed_db && ./run_server"]

