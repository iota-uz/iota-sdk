FROM golang:1.21.4

ENV GO111MODULE=auto \
    CGO_ENABLED=1 \
    GOOS=linux

WORKDIR /build

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o run_server cmd/server/main.go

CMD sql-migrate up -env="production" && /build/run_server
