# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# go.mod requires a recent 1.26.x; let the toolchain upgrade itself if the
# base image lags behind.
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY src ./src

ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w" \
    -o clipshare \
    ./src/cmd/clipshare

# Runtime stage
FROM alpine:3.21

# xclip + xvfb give the container a working clipboard via a virtual X server.
RUN apk add --no-cache ca-certificates xclip xvfb \
    && addgroup -S clipshare \
    && adduser -S -G clipshare clipshare \
    && mkdir -p /tmp/.X11-unix && chmod 1777 /tmp/.X11-unix

WORKDIR /app

COPY --from=builder /build/clipshare .
COPY config.toml .

RUN chown -R clipshare:clipshare /app

ENV CLIPSHARE_CONFIG=/app/config.toml
ENV DISPLAY=:99

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER clipshare

EXPOSE 40403/tcp 40405/tcp 40404/udp

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:40405/status >/dev/null || exit 1

ENTRYPOINT ["/entrypoint.sh"]
CMD ["./clipshare", "daemon"]
