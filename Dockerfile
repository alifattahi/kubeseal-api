# ── Stage 1: Download kubeseal ────────────────────────────────────────────────
FROM alpine:3.20 AS kubeseal-downloader

ARG KUBESEAL_VERSION=0.36.6
# remove this line if you want to download from GitHub releases
# COPY ./sealed-secret-files/kubeseal-0.36.6-linux-amd64.tar.gz /tmp/kubeseal.tar.gz 

# RUN ALPINE_VERSION=$(cat /etc/alpine-release | cut -d'.' -f1-2) && \
#     echo "https://mirror.arvancloud.ir/alpine/v${ALPINE_VERSION}/main" > /etc/apk/repositories && \
#     echo "https://mirror.arvancloud.ir/alpine/v${ALPINE_VERSION}/community" >> /etc/apk/repositories \
#     apk update
RUN apk add --no-cache curl tar && \
    curl -fsSL \
      "https://github.com/bitnami-labs/sealed-secrets/releases/download/v${KUBESEAL_VERSION}/kubeseal-${KUBESEAL_VERSION}-linux-arm64.tar.gz" \
      -o /tmp/kubeseal.tar.gz && \
    tar -xzf /tmp/kubeseal.tar.gz -C /usr/local/bin kubeseal && \
    chmod +x /usr/local/bin/kubeseal && \
    kubeseal --version

# ── Stage 2: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
# go.sum only if present (no external deps right now)
COPY go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sealed-secret-api .

# ── Stage 3: Minimal runtime ──────────────────────────────────────────────────
FROM alpine:3.20
# RUN ALPINE_VERSION=$(cat /etc/alpine-release | cut -d'.' -f1-2) && \
#     echo "https://mirror.arvancloud.ir/alpine/v${ALPINE_VERSION}/main" > /etc/apk/repositories && \
#     echo "https://mirror.arvancloud.ir/alpine/v${ALPINE_VERSION}/community" >> /etc/apk/repositories \
#     apk update

RUN apk add --no-cache ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

# kubeseal binary
COPY --from=kubeseal-downloader /usr/local/bin/kubeseal /usr/local/bin/kubeseal

# API binary
COPY --from=builder /sealed-secret-api /usr/local/bin/sealed-secret-api

# Cert is expected at /certs/sealed-secrets.pem — mount as a volume
VOLUME ["/certs"]

USER appuser
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/sealed-secret-api"]
