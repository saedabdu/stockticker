FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build the Go app
RUN go build -o stockticker ./cmd/stockticker

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/stockticker .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./stockticker"]