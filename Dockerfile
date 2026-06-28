# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build frontend
RUN apk add --no-cache nodejs npm
WORKDIR /app/frontend
RUN npm ci && npm run build

# Build binary
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux go build -o /mse-engine ./cmd/engine

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /mse-engine .
COPY --from=builder /app/frontend/dist ./frontend/dist

# Expose port
EXPOSE 8080

# Run
CMD ["./mse-engine"]
