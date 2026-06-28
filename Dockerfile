FROM golang:1.24-alpine AS builder
RUN apk add --no-cache nodejs npm
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd frontend && npm ci && npm run build
RUN CGO_ENABLED=0 go build -o /engine ./cmd/engine

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /engine /engine
EXPOSE 8080
ENTRYPOINT ["/engine"]
