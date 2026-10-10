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
FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/protomock /usr/local/bin/protomock

# Mocks are read from /app/mocks by default; mount your own folder there.
EXPOSE 8085 8086
ENTRYPOINT ["protomock"]
