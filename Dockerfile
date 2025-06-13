ARG ALPINE_VERSION=3.22.0

# Cross-compile unregistry for multiple architectures to speed up the build process on GitHub Actions.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /build

# Download and cache dependencies and only redownload them in subsequent builds if they change.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o unregistry ./cmd/unregistry


# Create a minimal image with the static binary built in the builder stage.
FROM alpine:${ALPINE_VERSION}

COPY --from=builder /build/unregistry /usr/local/bin/

EXPOSE 5000
# Run as root user by default to allow access to the containerd socket. This in unfortunate as running as non-root user
# requires changing the containerd socket permissions which still can be manually done by advanced users.
ENTRYPOINT ["unregistry"]

