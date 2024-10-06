FROM golang:1.21.4 as build-stage

ENV GO111MODULE=auto \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o run_server cmd/server/main.go


FROM build-stage as production-stage
CMD sql-migrate up -env="production" && /build/run_server

FROM build-stage as test-stage
CMD golangcli-lint run && go test -v ./...