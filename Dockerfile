FROM golang:1.23.2 as install-stage

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

FROM install-stage as production
COPY . .
RUN go build -o run_server cmd/server/main.go
CMD sql-migrate up -env="production" && /build/run_server

FROM install-stage as testing
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
COPY . .
CMD golangci-lint run --timeout 1000s && go test -v ./...
