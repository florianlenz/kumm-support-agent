# 1. Basis: Go Toolchain auf Debian
FROM golang:1.25-bookworm AS base

# 2. System-Abhängigkeiten
# Keine -dev-Pakete: Redis-/Postgres-Clients in Go sind pure Go.
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    unzip \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# ---

FROM base AS dev

# go.mod/go.sum entstehen erst im Container (go mod init/tidy),
# Dependencies werden über das go-mod-Volume gecacht.
EXPOSE 8080

# ---

FROM base AS ai

# Node.js aus dem offiziellen Image übernehmen (beide bookworm/glibc)
COPY --from=node:22-bookworm /usr/local/bin/node /usr/local/bin/node
COPY --from=node:22-bookworm /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    corepack enable 2>/dev/null || true

RUN curl -fsSL https://opencode.ai/install | bash && \
    OPENCODE_PATH=$(find /root -name opencode -type f | head -n 1) && \
    mv "$OPENCODE_PATH" /usr/local/bin/opencode && \
    chmod +x /usr/local/bin/opencode