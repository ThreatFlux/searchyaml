# syntax=docker/dockerfile:1

# Stage 1: Build the binaries
FROM golang:1.23 as builder

# Set the working directory
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the binaries for different architectures and OS
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/linux_amd64/searchYAML ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /out/linux_arm64/searchYAML ./...
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o /out/darwin_amd64/searchYAML ./...
RUN CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /out/darwin_arm64/searchYAML ./...

# Stage 2: Create a minimal image with the binaries
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binaries from the builder stage
COPY --from=builder /out/linux_amd64/searchYAML /app/linux_amd64/searchYAML
COPY --from=builder /out/linux_arm64/searchYAML /app/linux_arm64/searchYAML
COPY --from=builder /out/darwin_amd64/searchYAML /app/darwin_amd64/searchYAML
COPY --from=builder /out/darwin_arm64/searchYAML /app/darwin_arm64/searchYAML

# Set the entrypoint to a shell to explore the binaries
ENTRYPOINT ["/bin/sh"]