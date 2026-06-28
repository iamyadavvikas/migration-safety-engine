# Stage 1: Build Frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go Engine
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy compiled frontend from Stage 1
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

RUN CGO_ENABLED=0 GOOS=linux go build -o /mse-engine ./cmd/engine

# Stage 3: Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /mse-engine .
COPY --from=backend-builder /app/frontend/dist ./frontend/dist

EXPOSE 8080

CMD ["./mse-engine"]
