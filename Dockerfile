FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o tgifreezeday ./cmd/tgifreezeday

# Use a small alpine image for the final container
FROM alpine:3.18

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/tgifreezeday /app/tgifreezeday

# Create a directory for the config
RUN mkdir -p /app/config

# Set execution permissions
RUN chmod +x /app/tgifreezeday

# Set the entrypoint
ENTRYPOINT ["/app/tgifreezeday", "--config", "/app/config/config.yaml"]

# Use a non-root user for security
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser 