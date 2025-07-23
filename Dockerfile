# Dockerfile
FROM golang:1.24.5-alpine

# Install necessary tools
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go.mod and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the binary
RUN go build -o protomock ./cmd/server

# Set entrypoint to run the built binary
ENTRYPOINT ["./protomock"]
