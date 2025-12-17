FROM golang:1.21-alpine AS build-env

# Accept proxy settings as build arguments
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY

# Set proxy environment variables
ENV HTTP_PROXY=${HTTP_PROXY}
ENV HTTPS_PROXY=${HTTPS_PROXY}
ENV NO_PROXY=${NO_PROXY}
ENV http_proxy=${HTTP_PROXY}
ENV https_proxy=${HTTPS_PROXY}
ENV no_proxy=${NO_PROXY}

# Install git (needed for go get)
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 go build -o goapp

# final stage
FROM alpine:latest
WORKDIR /app
COPY --from=build-env /src/goapp .
COPY --from=build-env /src/md.html .
COPY --from=build-env /src/content.html* ./
ENTRYPOINT ["./goapp"]