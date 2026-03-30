FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o meadow ./cmd/meadow

FROM alpine:3.21
RUN apk add --no-cache ttyd
COPY --from=builder /build/meadow /app/meadow
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh
USER 1000:1000
ENTRYPOINT ["/app/entrypoint.sh"]
