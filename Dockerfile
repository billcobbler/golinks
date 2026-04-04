# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static binary. CGO is not needed because we use modernc.org/sqlite.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /golinks-server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -H -u 1000 golinks && \
    mkdir -p /data && chown golinks:golinks /data

WORKDIR /data
USER golinks

COPY --from=builder /golinks-server /usr/local/bin/golinks-server

EXPOSE 8080

# Mount a volume at /data to persist the SQLite database.
VOLUME ["/data"]

ENV GOLINKS_DB=/data/golinks.db

ENTRYPOINT ["/usr/local/bin/golinks-server"]
