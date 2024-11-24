FROM golang:1.23.2 AS install-stage

WORKDIR /build

RUN go install github.com/rubenv/sql-migrate/...@latest
RUN go install github.com/a-h/templ/cmd/templ@latest
COPY go.mod go.sum ./
RUN go mod download

FROM install-stage AS production
COPY . .
RUN templ generate
RUN go build -o run_server cmd/server/main.go
CMD sql-migrate up -env="production" && /build/run_server

FROM install-stage AS staging
COPY . .
RUN templ generate
RUN go build -o run_server cmd/server/main.go && go build -o seed_db cmd/seed/main.go
CMD sql-migrate up -env="staging" && /build/seed_db && /build/run_server


FROM install-stage AS testing
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
COPY . .
RUN templ generate
#CMD golangci-lint run && go test -v ./...
CMD go test -v ./...
