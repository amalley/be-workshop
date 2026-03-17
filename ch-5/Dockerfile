ARG GO_IMAGE="1.26.1-alpine"

# Stage 1: Build the Go application
FROM golang:${GO_IMAGE} AS builder

# Set our working directory
WORKDIR /app

# Copy the mod and sum files into our working directory
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Now copy everything else (overwritting mod and sum, that's fine) into our working directory
# Note: the .dockerignore will ensure only what we want copied is allowed to be. You may see warnings in the build log
COPY . ./

# Finally build our application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wikistats-consumer ./cmd/consumer/main.go

# Stage 2: Build the minimal runtime image
FROM alpine:latest

# Set our working directory
WORKDIR /app

# Copy the binary from build stage
COPY --from=builder /app/wikistats-consumer ./

# Set the entry point as the application
ENTRYPOINT ["/app/wikistats-consumer"]