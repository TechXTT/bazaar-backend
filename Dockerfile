# syntax=docker/dockerfile:1

# INFRA-1: production image for bazaar-backend.
#
# Multi-stage build:
#   1. `builder` compiles a fully static Go 1.21 binary (CGO disabled — the app
#      has no cgo deps, so the binary runs on a scratch/distroless base).
#   2. final stage is a distroless static image that runs the binary as the
#      non-root `nonroot` user.
#
# Runtime files the app reads at startup are baked in:
#   * Escrow.json  — observer reads "./Escrow.json" (see main.go) for the ABI.
# Secrets (.env, *.pem, .dev/ JWT keys) are NEVER baked: they arrive via the
# root .env (env_file) and the mounted /data volume at runtime (see compose).

# ---- build stage ----
FROM golang:1.21 AS builder

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

# Build the binary.
COPY . .
# CGO_ENABLED=0 -> static binary usable on distroless/static.
# -trimpath + -ldflags "-s -w" produce a smaller, reproducible binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/bazaar-backend .

# ---- final stage ----
# distroless/static: no shell, no package manager, minimal attack surface.
# The :nonroot tag runs as uid/gid 65532 by default.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Compiled binary.
COPY --from=builder /out/bazaar-backend /app/bazaar-backend

# Runtime data the app reads relative to its working dir.
COPY --from=builder /src/Escrow.json /app/Escrow.json

# Note: LOCAL_UPLOAD_DIR resolves to /app/uploads. In compose that path is a
# mounted volume (backend-uploads) owned/created at runtime, so we don't bake an
# empty dir here (distroless has no shell to mkdir, and an empty dir adds nothing).

USER nonroot:nonroot

# HTTP_PORT defaults to 8000 (config.go); server binds ":8000".
EXPOSE 8000

ENTRYPOINT ["/app/bazaar-backend"]
