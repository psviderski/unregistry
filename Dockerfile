ARG ALPINE_VERSION=3.22.0

FROM golang:1.24-alpine AS builder

WORKDIR /build

# Download and cache dependencies and only redownload them in subsequent builds if they change.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o unregistry ./cmd/unregistry


# Unregistry in Docker-in-Docker image for e2e tests.
FROM docker:28.2.2-dind AS unregistry-dind

ENV UNREGISTRY_CONTAINERD_SOCK="/run/docker/containerd/containerd.sock"
# dind uses 'default' namespace for containerd by default instead of 'moby' used by Docker Engine.
ENV UNREGISTRY_CONTAINERD_NAMESPACE="default"

COPY scripts/dind-entrypoint.sh /usr/local/bin/entrypoint.sh
COPY --from=builder /build/unregistry /usr/local/bin/

EXPOSE 5000
ENTRYPOINT ["entrypoint.sh"]
CMD ["unregistry"]

# Create a minimal image with the static binary built in the builder stage.
FROM alpine:${ALPINE_VERSION}

COPY --from=builder /build/unregistry /usr/local/bin/

EXPOSE 5000
# Run as root user by default to allow access to the containerd socket. This in unfortunate as running as non-root user
# requires changing the containerd socket permissions which still can be manually done by advanced users.
ENTRYPOINT ["unregistry"]

