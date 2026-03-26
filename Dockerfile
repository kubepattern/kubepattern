# Stage 1: Build the binary
FROM golang:alpine AS builder

LABEL authors="gabrielegroppo"
WORKDIR /app
COPY go.mod ./
# COPY go.sum ./
RUN go mod download
COPY . .

# Build the application
# We target the main.go file in cmd/kubepattern
RUN CGO_ENABLED=0 GOOS=linux go build -o kubepattern ./cmd/kubepattern/main.go

# Stage 2: Final lightweight image
FROM alpine:latest

# Install ca-certificates in case your tool makes HTTPS calls
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/kubepattern .

# Expose the port (matching your server.go logic)
EXPOSE 8090

# Run the binary
ENTRYPOINT ["./kubepattern"]
