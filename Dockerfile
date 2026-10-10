# Build stage: compile a static binary (cross-compiled natively for multi-arch builds).
FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/protomock ./cmd/server

# Runtime stage: just the binary (the UI is embedded in it).
# curl is included for docker-compose healthchecks that call a mock URL.
FROM alpine:3.20
RUN apk add --no-cache curl
WORKDIR /app
COPY --from=build /out/protomock /usr/local/bin/protomock

# Mocks are read from /app/mocks by default; mount your own folder there.
EXPOSE 8085 8086

# Healthy once both servers answer (and, with PROTOMOCK_STRICT=true, every mock loaded).
# It reads the same PROTOMOCK_* env vars as the server, so custom ports are checked too.
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 CMD ["protomock", "healthcheck"]

ENTRYPOINT ["protomock"]
