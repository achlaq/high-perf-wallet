# Stage 1: Build stage
FROM golang:1.26.3-alpine AS builder

# Set working directory
WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./

# Cache Go modules
RUN go mod download

# Copy source code
COPY . .

# Compile Go binary as a statically linked execution file
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/api/main.go

# Stage 2: Final minimal run stage
FROM alpine:3.19

WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /app/main .

# Copy environment file config
COPY --from=builder /app/.env .

# Expose the API port
EXPOSE 8080

# Command to run the application
CMD ["./main"]
