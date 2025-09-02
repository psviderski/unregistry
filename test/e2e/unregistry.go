package e2e

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// runUnregistryDinD starts unregistry in a Docker-in-Docker container. It returns the mapped Docker
// port and the mapped unregistry port. The containerdStore parameter specifies whether to use containerd image store.
func runUnregistryDinD(t *testing.T, containerdStore bool) (string, string) {
	ctx := context.Background()
	// Start unregistry in a Docker-in-Docker container with Docker using containerd image store.
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join("..", ".."),
				Dockerfile: "Dockerfile.test",
				BuildOptionsModifier: func(buildOptions *types.ImageBuildOptions) {
					buildOptions.Target = "unregistry-dind"
				},
			},
			Env: map[string]string{
				"DOCKER_CONTAINERD_STORE": fmt.Sprintf("%t", containerdStore),
				"UNREGISTRY_LOG_LEVEL":    "debug",
			},
			Privileged: true,
			// Explicitly specify the host port for the registry because if not specified, 'docker push' from Docker
			// Desktop is unable to reach the automatically mapped one for some reason.
			ExposedPorts: []string{"2375", "50000:5000"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("2375"),
				wait.ForListeningPort("5000"),
			).WithStartupTimeoutDefault(15 * time.Second),
		},
		Started: true,
	}
	ctr, err := testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)

	t.Cleanup(func() {
		// Print last 20 lines of unregistry container logs.
		logs, err := ctr.Logs(ctx)
		assert.NoError(t, err, "Failed to get logs from unregistry container.")
		if err == nil {
			defer logs.Close()
			logsContent, err := io.ReadAll(logs)
			assert.NoError(t, err, "Failed to read logs from unregistry container.")
			if err == nil {

				lines := strings.Split(string(logsContent), "\n")
				start := len(lines) - 20
				if start < 0 {
					start = 0
				}

				t.Log("=== Last 20 lines of unregistry container logs ===")
				for i := start; i < len(lines); i++ {
					if lines[i] != "" {
						t.Log(lines[i])
					}
				}
				t.Log("=== End of unregistry container logs ===")
			}
		}

		// Ensure the container is terminated after the test.
		assert.NoError(t, ctr.Terminate(ctx))
	})

	mappedDockerPort, err := ctr.MappedPort(ctx, "2375")
	require.NoError(t, err)
	mappedRegistryPort, err := ctr.MappedPort(ctx, "5000")
	require.NoError(t, err)

	return mappedDockerPort.Port(), mappedRegistryPort.Port()
}
