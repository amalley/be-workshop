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
COPY . ./

# Finally build our application
RUN CGO_ENABLED=0 GOOS=linux go build -o wikistats ./cmd/main.go

# Stage 2: Build the minimal runtime image
FROM alpine:latest

# Set out working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/wikistats .

# Set the entry point as the application
ENTRYPOINT ["/app/wikistats"]

# Set a default port for the application to listen on (can be overriden with --port when running the image)
CMD ["--port", "7000"]