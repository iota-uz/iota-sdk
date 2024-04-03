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
RUN go build -o migrate cmd/migrate/main.go

CMD /build/migrate up && /build/run_server
