package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerPusshPlugin tests the happy path of pushing an image to a remote host using the docker-pussh plugin.
// It tests both the enabled and disabled containerd image store on the remote host.
func TestDockerPusshPlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Create local Docker client.
	localCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer localCli.Close()

	// Check if local Docker uses containerd image store: https://docs.docker.com/engine/storage/containerd/
	info, err := localCli.Info(ctx)
	require.NoError(t, err)
	localDockerUsesContainerdImageStore := strings.Contains(
		fmt.Sprintf("%s", info.DriverStatus), "containerd.snapshotter",
	)

	imageName := "traefik/whoami:v1.10.3"
	platform := "linux/amd64"
	indexDigest := "sha256:43a68d10b9dfcfc3ffbfe4dd42100dc9aeaf29b3a5636c856337a5940f1b4f1c"
	platformDigest := "sha256:c899811bc4a1f63a1273c612e15f1bea6514a19c7b08143dbbdef3e8f882c38d"
	// Local image digest for the platform when *not* using containerd image store.
	dockerLocalDigest := "sha256:aeef15490f2bf3144bff9167ee46eb7d9f8f072ab2c16c563bc45b0eeae3d707"

	t.Cleanup(func() {
		_, err = localCli.ImageRemove(ctx, imageName, image.RemoveOptions{PruneChildren: true})
		if !client.IsErrNotFound(err) {
			assert.NoError(t, err)
		}
	})

	require.NoError(
		t, pullImage(ctx, localCli, imageName, image.PullOptions{Platform: platform}),
		"Failed to pull image '%s' locally", imageName,
	)
	img, _, err := localCli.ImageInspectWithRaw(ctx, imageName)
	require.NoError(t, err, "Failed to inspect image '%s' locally", imageName)
	if localDockerUsesContainerdImageStore {
		require.Equal(t, indexDigest, img.ID, "Image ID should match OCI index digest")
	} else {
		require.Equal(t, dockerLocalDigest, img.ID, "Image ID should match local Docker image digest")
	}

	tests := []struct {
		name            string
		registryPort    int
		containerdStore bool
	}{
		{
			name:            "native image store",
			registryPort:    50001,
			containerdStore: false,
		},
		{
			name:            "containerd image store",
			registryPort:    50002,
			containerdStore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dockerPort, sshPort := runUnregistryDinD(t, tt.registryPort, tt.containerdStore)

			// Create remote Docker client.
			remoteCli, err := client.NewClientWithOpts(
				client.WithHost("tcp://localhost:"+dockerPort),
				client.WithAPIVersionNegotiation(),
			)
			require.NoError(t, err)
			defer remoteCli.Close()

			// Ensure image doesn't exist on remote before pushing.
			_, _, err = remoteCli.ImageInspectWithRaw(ctx, imageName)
			require.Error(t, err, "Image should not exist on remote before pushing")

			root := projectRoot()
			dockerPusshPath := filepath.Join(root, "docker-pussh")
			sshKeyPath := filepath.Join(root, "test/e2e/ssh/test_key")
			cmd := exec.Command(dockerPusshPath,
				"-i", sshKeyPath,
				"--no-host-key-check",
				imageName,
				fmt.Sprintf("root@localhost:%s", sshPort),
			)

			t.Logf("Running docker-pussh command: %s", cmd.String())

			output, err := cmd.CombinedOutput()
			t.Logf("docker-pussh output:\n%s", string(output))
			require.NoError(t, err, "Failed to run docker-pussh")

			// Verify the image now exists on the remote Docker.
			remoteImg, _, err := remoteCli.ImageInspectWithRaw(ctx, imageName)
			require.NoError(t, err, "Pushed image should appear in the remote Docker")

			// Verify the image details match expectations based on containerd store.
			if tt.containerdStore {
				assert.Equal(t, platformDigest, remoteImg.ID, "Image ID should match platform-specific image digest")
			} else {
				assert.Equal(t, dockerLocalDigest, remoteImg.ID, "Image ID should match Docker local image digest")
			}

			// Push the same image again to verify idempotency.
			cmd = exec.Command(dockerPusshPath,
				"-i", sshKeyPath,
				"--no-host-key-check",
				imageName,
				fmt.Sprintf("root@localhost:%s", sshPort),
			)

			output, err = cmd.CombinedOutput()
			t.Logf("docker-pussh output (second push):\n%s", string(output))
			require.NoError(t, err, "Failed to run docker-pussh second time: %s", string(output))

			// Verify the image still exists and hasn't changed.
			remoteImg2, _, err := remoteCli.ImageInspectWithRaw(ctx, imageName)
			require.NoError(t, err, "Image should still exist on remote after second push")
			assert.Equal(t, remoteImg.ID, remoteImg2.ID, "Image ID should remain the same after second push")
		})
	}
}

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
