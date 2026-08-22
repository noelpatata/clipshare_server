FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o listener ./main.go

FROM alpine:3.21

COPY --from=builder /build/listener /usr/local/bin/listener

ENTRYPOINT ["/usr/local/bin/listener"]
