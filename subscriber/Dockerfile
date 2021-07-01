# Using Multi-stage build process to minify final api container
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds

### STEP 1: build executable binary

# Based off the Golang Dockerfile example
# https://hub.docker.com/_/golang

# Use the official Golang runtime as a parent image
FROM golang:latest AS builder

# Set the current working directory
WORKDIR /go/src/app

# Copy the current directory contents into the container
COPY . .

# Download and install necessary Golang dependencies
RUN go mod download

# Build api executable (to be used in next stage)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o /go/bin/api .


### STEP 2: Copy executable into a new smaller image

# Use the alpine linux runtime as a parent image
FROM alpine:latest

# Set the current working directory
WORKDIR /root/

# add certificates
RUN apk --no-cache add ca-certificates

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /go/bin .

# Give the executable permission to run
RUN chmod +x ./api

# Configure container to run as api executable
ENTRYPOINT ["./api"]

# Expose port 8080 to the outside world
EXPOSE 8080