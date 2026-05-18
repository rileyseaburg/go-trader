# Single-container build: React (Vite) → Rust binary serves the SPA + API.
# The Rust Axum process is the only network-facing thing in the container;
# its static-file service serves the React bundle from /app.

# ──────────────────────────────────────────────────────────────────────
# Stage 1 — build the React bundle
# ──────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS web

WORKDIR /web
# Copy package files first so npm install can be cached separately from
# source changes.
COPY web-ui/package.json web-ui/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY web-ui/ ./
RUN npm run build

# ──────────────────────────────────────────────────────────────────────
# Stage 2 — build the Rust binary
# ──────────────────────────────────────────────────────────────────────
FROM rust:1.95-bookworm AS rustbuild

WORKDIR /src/rust-trader

# Copy the Rust workspace. Keep rust-trader/target out of the Docker context
# via .dockerignore so local build artifacts do not bloat the context.
COPY rust-trader/ ./

# The release binary is dynamically linked to glibc but uses rustls for TLS,
# so no OpenSSL shared libraries are required in the runtime image.
RUN cargo build --release --locked --bin go-trader

# ──────────────────────────────────────────────────────────────────────
# Stage 3 — runtime
# ──────────────────────────────────────────────────────────────────────
# Debian slim provides glibc for the Rust binary. CA certs are included so
# HTTPS calls to FRED/Alpaca/Vault work out of the box.
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Run as distroless-compatible nonroot UID/GID previously used by the K8s
# manifest, while keeping /app/data writable for basket/state files.
RUN groupadd --system --gid 65532 nonroot \
 && useradd --system --uid 65532 --gid 65532 --home-dir /app --shell /usr/sbin/nologin nonroot \
 && mkdir -p /app/data \
 && chown -R 65532:65532 /app

# Bring in the Rust binary and the React bundle. The bundle is laid out at
# the workdir root so Axum serves index.html at "/" and Vite's hashed assets
# at "/assets/...".
COPY --from=rustbuild /src/rust-trader/target/release/go-trader /app/go-trader
COPY --from=web       /web/dist/                                /app/
COPY --chown=65532:65532 data/ /app/data/

EXPOSE 8080
USER 65532:65532

ENTRYPOINT ["/app/go-trader"]
CMD ["--port", "8080"]
