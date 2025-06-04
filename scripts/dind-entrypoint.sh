#!/bin/sh
set -eu

# Cleanup function to properly terminate background processes.
cleanup() {
    echo "Terminating container processes..."

    # Terminate the main process if it has been started.
    if [ -n "${MAIN_PID:-}" ]; then
        kill "$MAIN_PID" 2>/dev/null || true
    fi

    # Terminate Docker daemon if PID file exists.
    if [ -f /run/docker.pid ]; then
        kill "$(cat /run/docker.pid)" 2>/dev/null || true
    fi

    # Wait for processes to terminate.
    wait
}
trap cleanup INT TERM EXIT

if [ "${DOCKER_CONTAINERD_STORE:-true}" = "true" ]; then
    echo "Using containerd image store for Docker."
    mkdir -p /etc/docker
    echo '{"features": {"containerd-snapshotter": true}}' > /etc/docker/daemon.json
else
    echo "Using the default Docker image store."
fi

dind dockerd --host=tcp://0.0.0.0:2375 --tls=false &

# Execute the passed command and wait for it while maintaining signal handling.
"$@" &
MAIN_PID=$!
wait $MAIN_PID
