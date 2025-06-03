ARG ALPINE_VERSION=3.22.0

FROM golang:1.24-alpine AS builder

WORKDIR /build

# Download and cache dependencies and only redownload them in subsequent builds if they change.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o unregistry ./cmd/unregistry


# Create a minimal image with the static binary built in the builder stage.
FROM alpine:${ALPINE_VERSION}

# TODO: remove when migrated to containerd storage.
RUN mkdir -p /var/lib/unregistry

COPY --from=builder /build/unregistry /usr/local/bin/
# Run as root user by default to allow access to the containerd socket. This in unfortunate as running as non-root user
# requires changing the containerd socket permissions which still can be manually done by advanced users.
ENTRYPOINT ["unregistry"]
