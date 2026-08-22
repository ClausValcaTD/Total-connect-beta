# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make protoc

# Copy go mod files
COPY go.work go.work.sum ./
COPY api/proto/go.mod api/proto/go.sum ./api/proto/
COPY bridge/go.mod bridge/go.sum ./bridge/
COPY cli/go.mod cli/go.sum ./cli/
COPY core/go.mod core/go.sum ./core/

# Download dependencies
RUN go work sync

# Copy source code
COPY . .

# Build binaries
RUN make build

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/tc-server .
COPY --from=builder /app/tc-cli .

# Expose gRPC port
EXPOSE 50051

# Default command
CMD ["./tc-server"]
