# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/cli ./cmd/cli

# Worker image
FROM alpine:3.19 AS worker

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/worker /usr/local/bin/worker

USER nobody
ENTRYPOINT ["/usr/local/bin/worker"]

# API image
FROM alpine:3.19 AS api

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/api /usr/local/bin/api

USER nobody
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

# CLI image
FROM alpine:3.19 AS cli

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/cli /usr/local/bin/cli

ENTRYPOINT ["/usr/local/bin/cli"]
