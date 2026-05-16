# Stage 1 — build SvelteKit frontend
# Built on the build platform because frontend output is arch-independent.
FROM --platform=$BUILDPLATFORM node:24-alpine3.23 AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Stage 2 — build Go binary (static, no CGO)
FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine3.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overlay the compiled SPA produced by Stage 1 before Go compilation so embed.FS sees it.
COPY --from=frontend /app/internal/webui/dist/ internal/webui/dist/
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG TARGETVARIANT=
RUN case "${TARGETARCH}" in \
      arm) export GOARM="${TARGETVARIANT#v}" ;; \
    esac && \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" ${GOARM:+GOARM="${GOARM}"} \
    go build -ldflags="-s -w" -o gomeshcomd ./cmd/gomeshcomd

# Stage 3 — minimal runtime (static binary, no shell)
FROM alpine:3.23
COPY --from=builder /app/gomeshcomd /gomeshcomd

VOLUME ["/data"]

ENV GOMESHCOM_HTTP_ADDR=0.0.0.0:8080
ENV GOMESHCOM_DATA_DIR=/data
ENV GOMESHCOM_RECEIVE_LOG_PATH=/data/raw

# HTTP UI
EXPOSE 8080
# MeshCom UDP bridge
EXPOSE 1799/udp

ENTRYPOINT ["/gomeshcomd"]
