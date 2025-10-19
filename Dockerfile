# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy source code (including vendor directory)
COPY . .

# Build the application using vendored dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o main .

# Final stage
FROM scratch

# Copy the binary from builder
COPY --from=builder /app/main /main

# Expose port
EXPOSE 8080

# Run the application
CMD ["/main"]
