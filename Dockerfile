# Build stage
# Use AWS ECR Public mirror to avoid docker.io rate limits or mirror issues
FROM public.ecr.aws/docker/library/golang:alpine AS builder

# Install git for fetching dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
RUN go build -o main ./cmd/main.go

# Final stage
FROM public.ecr.aws/docker/library/alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS calls (if needed for external APIs)
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Copy default config (can be overridden by volume mount)
COPY config.toml .

# Expose the application port
EXPOSE 9000

# Run the binary
CMD ["./main"]