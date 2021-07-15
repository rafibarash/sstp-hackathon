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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o /go/bin/frontend .


### STEP 2: Copy executable into a new smaller image

# Use the alpine linux runtime as a parent image
FROM alpine:latest

### Stage 2 - the production environment
FROM us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/nginx:latest

# Set the current working directory
WORKDIR /root/

# add certificates
RUN apk --no-cache add ca-certificates

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /go/bin .
COPY --from=builder /go/src/app/layout.html .

# Give the executable permission to run
RUN chmod +x ./frontend

# Configure container to run as watcher executable
ENTRYPOINT ["./frontend"]

# Expose port 8080 to the outside world
EXPOSE 8080