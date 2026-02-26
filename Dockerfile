# syntax=docker/dockerfile:1

FROM golang:1.24-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git build-essential && rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1
ENV GOFLAGS=""

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -tags "fts5" -o /out/claude-mnemonic-worker ./cmd/worker

FROM debian:bookworm-slim AS runtime

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tini && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/claude-mnemonic-worker /usr/local/bin/claude-mnemonic-worker

VOLUME ["/data"]

ENV CLAUDE_MNEMONIC_DB_PATH=/data/claude-mnemonic.db
ENV CLAUDE_MNEMONIC_WORKER_HOST=0.0.0.0
ENV CLAUDE_MNEMONIC_WORKER_PORT=37777
EXPOSE 37777

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/usr/local/bin/claude-mnemonic-worker"]
