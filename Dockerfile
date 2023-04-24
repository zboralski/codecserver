# Use the official Golang image as the base image
FROM golang:1.20.3-alpine as builder
WORKDIR /app

# Download all dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build the binary
COPY . .
RUN go build -o codecserver github.com/zboralski/codecserver

# Start a new stage to create a smaller final image
FROM alpine:latest
WORKDIR /app

# Copy the binary from the builder stage into the final stage
COPY --from=builder /app/codecserver /app/codecserver

# Expose the application's port
EXPOSE 8081

# Start the application
CMD ["/app/codecserver"]
