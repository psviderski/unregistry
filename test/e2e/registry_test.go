package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/jsonmessage"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRegistryPushPull(t *testing.T) {
	ctx := context.Background()

	// Start unregistry in a Docker-in-Docker container with Docker using containerd image store.
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context: filepath.Join("..", ".."),
				BuildOptionsModifier: func(buildOptions *types.ImageBuildOptions) {
					buildOptions.Target = "unregistry-dind"
				},
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
	unregistryContainer, err := testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			// Ensure the container is terminated after the test.
			assert.NoError(t, unregistryContainer.Terminate(ctx))
		},
	)

	mappedDockerPort, err := unregistryContainer.MappedPort(ctx, "2375")
	require.NoError(t, err)
	mappedRegistryPort, err := unregistryContainer.MappedPort(ctx, "5000")
	require.NoError(t, err)

	remoteCli, err := client.NewClientWithOpts(
		client.WithHost("tcp://localhost:"+mappedDockerPort.Port()),
		client.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)
	defer remoteCli.Close()

	registryAddr := "localhost:" + mappedRegistryPort.Port()
	t.Logf("Unregistry started at %s", registryAddr)

	localCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer localCli.Close()

	// This test expects local Docker to use containerd image store: https://docs.docker.com/engine/storage/containerd/
	t.Run(
		"push single-platform image", func(t *testing.T) {
			t.Parallel()

			imageName := "busybox:1.37.0-musl"
			registryImage := fmt.Sprintf("%s/%s", registryAddr, imageName)
			platform := "linux/amd64"
			ociPlatform := ocispec.Platform{Architecture: "amd64", OS: "linux"}
			platformDigest := "sha256:008f65c96291274170bec5cf01b2de06dc049dc9d8f9bfb633520497875ed2c1"

			t.Cleanup(
				func() {
					for _, img := range []string{imageName, registryImage} {
						_, err := localCli.ImageRemove(ctx, img, image.RemoveOptions{PruneChildren: true})
						assert.NoError(t, err)
					}
				},
			)

			require.NoError(t, pullImage(ctx, localCli, imageName, image.PullOptions{Platform: platform}))

			require.NoError(t, localCli.ImageTag(ctx, imageName, registryImage))
			require.NoError(t, pushImage(ctx, localCli, registryImage, image.PushOptions{Platform: &ociPlatform}))

			img, _, err := remoteCli.ImageInspectWithRaw(ctx, imageName)
			require.NoError(t, err)
			assert.Equal(t, platformDigest, img.ID, "Image ID should match the platform-specific image digest.")
		},
	)

	//t.Run(
	//	"push/pull multi platform", func(t *testing.T) {
	//		t.Parallel()
	//
	//		// A minimal image that contains only a few platforms.
	//		imageName := "traefik/whoami:v1.10.0"
	//		platforms := []string{"linux/amd64", "linux/arm64", "linux/arm/v7"}
	//	},
	//)
}

func pullImage(ctx context.Context, cli *client.Client, imageName string, opts image.PullOptions) error {
	respBody, err := cli.ImagePull(ctx, imageName, opts)
	if err != nil {
		return err
	}
	defer respBody.Close()

	decoder := json.NewDecoder(respBody)
	errCh := make(chan error, 1)

	go func() {
		var jm jsonmessage.JSONMessage
		for {
			if err = decoder.Decode(&jm); err != nil {
				if errors.Is(err, io.EOF) {
					errCh <- nil
					return
				}
				errCh <- fmt.Errorf("decode image pull message: %v", err)
				return
			}

			if jm.Error != nil {
				errCh <- fmt.Errorf("pull failed for '%s': %s", imageName, jm.Error.Message)
				return
			}
		}
	}()

	for {
		select {
		case err = <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func pushImage(ctx context.Context, cli *client.Client, imageName string, opts image.PushOptions) error {
	if opts.RegistryAuth == "" {
		opts.RegistryAuth = base64.URLEncoding.EncodeToString([]byte("{}"))
	}

	respBody, err := cli.ImagePush(ctx, imageName, opts)
	if err != nil {
		return err
	}
	defer respBody.Close()

	decoder := json.NewDecoder(respBody)
	errCh := make(chan error, 1)

	go func() {
		var jm jsonmessage.JSONMessage
		for {
			if err = decoder.Decode(&jm); err != nil {
				if errors.Is(err, io.EOF) {
					errCh <- nil
					return
				}
				errCh <- fmt.Errorf("decode image push message: %v", err)
				return
			}

			if jm.Error != nil {
				errCh <- fmt.Errorf("push failed for '%s': %s", imageName, jm.Error.Message)
				return
			}
		}
	}()

	for {
		select {
		case err = <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
