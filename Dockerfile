# Stage 1: Build the binary
FROM golang:1.25.8-alpine AS builder

LABEL authors="gabrielegroppo"
WORKDIR /app

# Install CA certificates (required for K8s API/HTTPS) and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Download dependencies (cached if go.mod/go.sum remain unchanged)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build a static binary, stripping debug info (-w -s) for a smaller footprint
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o kubepattern ./cmd/kubepattern/main.go

# Stage 2: Final zero-CVE image
FROM scratch

# Import certificates and timezone data from the builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

ENV ZONEINFO=/usr/share/zoneinfo
WORKDIR /app

COPY --from=builder /app/kubepattern .

# Run as non-root user (UID 65532) for Kubernetes security compliance
USER 65532:65532

ENTRYPOINT ["./kubepattern"]