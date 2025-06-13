package conformance

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

// SetupUnregistry starts unregistry in a Docker-in-Docker testcontainer.
func SetupUnregistry(t *testing.T) (testcontainers.Container, string) {
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
				"UNREGISTRY_LOG_LEVEL": "debug",
			},
			Privileged:   true,
			ExposedPorts: []string{"5000"},
			WaitingFor:   wait.ForListeningPort("5000").WithStartupTimeout(15 * time.Second),
		},
		Started: true,
	}

	ctr, err := testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)

	mappedRegistryPort, err := ctr.MappedPort(ctx, "5000")
	require.NoError(t, err)

	url := fmt.Sprintf("http://localhost:%s", mappedRegistryPort.Port())
	t.Logf("Unregistry started at %s", url)

	return ctr, url
}

// TeardownUnregistry cleans up the unregistry container.
func TeardownUnregistry(t *testing.T, ctr testcontainers.Container) {
	ctx := context.Background()

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
}
