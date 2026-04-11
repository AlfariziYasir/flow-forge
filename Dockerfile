# Build Stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the main server binary
RUN CGO_ENABLED=0 GOOS=linux go build -o flow-forge ./cmd/server/main.go
# Build the seeder binary
RUN CGO_ENABLED=0 GOOS=linux go build -o seed ./cmd/seed/main.go

# Run Stage
FROM alpine:latest

WORKDIR /app

# Copy the server and seeder binaries
COPY --from=builder /app/flow-forge .
COPY --from=builder /app/seed .

# Use non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8080

CMD ["./flow-forge"]
