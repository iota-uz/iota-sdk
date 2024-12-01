FROM golang:1.23.2 AS install-stage

WORKDIR /build

RUN case "$(uname -m)" in \
      "x86_64") ARCH="x64" ;; \
      "aarch64") ARCH="arm64" ;; \
      *) echo "Unsupported architecture: $(uname -m)" && exit 1 ;; \
    esac && \
    curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/v3.4.15/download/tailwindcss-linux-${ARCH} && \
    chmod +x tailwindcss-linux-${ARCH} && \
    mv tailwindcss-linux-${ARCH} /usr/local/bin/tailwindcss

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
RUN go install github.com/a-h/templ/cmd/templ@latest
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN templ generate && go vet ./...
RUN tailwindcss -c tailwind.config.js -i pkg/presentation/assets/css/main.css -o pkg/presentation/assets/css/main.min.css --minify

FROM install-stage AS production
RUN go build -o run_server cmd/server/main.go
CMD go run cmd/migrate/main.go up && /build/run_server

FROM install-stage AS staging
RUN go build -o run_server cmd/server/main.go && go build -o seed_db cmd/seed/main.go
CMD go run cmd/migrate/main.go up && /build/seed_db && /build/run_server

FROM install-stage AS testing
#CMD golangci-lint run && go test -v ./...
CMD go test -v ./...
