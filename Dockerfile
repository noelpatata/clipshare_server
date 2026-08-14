# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o clipshare \
    ./src/cmd/clipshare

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/clipshare .

# Default ports exposed by clipshare. When running the container, set
# [server] bind = "0.0.0.0" and [api] bind = "0.0.0.0" in your config.toml
# so the daemon listens on all interfaces.
EXPOSE 40403/tcp
EXPOSE 40405/tcp
EXPOSE 40404/udp

ENTRYPOINT ["./clipshare"]
CMD ["daemon"]
