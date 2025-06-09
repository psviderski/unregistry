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
	"strings"
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
			Env: map[string]string{
				"UNREGISTRY_LOG_LEVEL": "debug",
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
			// Print last 20 lines of unregistry container logs.
			logs, err := unregistryContainer.Logs(ctx)
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

	// Check if local Docker uses containerd image store: https://docs.docker.com/engine/storage/containerd/
	info, err := localCli.Info(ctx)
	require.NoError(t, err)
	localDockerUsesContainerdImageStore := strings.Contains(
		fmt.Sprintf("%s", info.DriverStatus), "containerd.snapshotter",
	)

	t.Run(
		"push/pull single-platform image", func(t *testing.T) {
			t.Parallel()

			imageName := "busybox:1.37.0-musl"
			registryImage := fmt.Sprintf("%s/%s", registryAddr, imageName)
			platform := "linux/amd64"
			ociPlatform := ocispec.Platform{Architecture: "amd64", OS: "linux"}
			indexDigest := "sha256:597bf7e5e8faf26b8efc4cb558eea5dc14d9cc97d5b4c8cdbe6404a7432d5a67"
			platformDigest := "sha256:008f65c96291274170bec5cf01b2de06dc049dc9d8f9bfb633520497875ed2c1"
			// Local image digest for the platform when *not* using containerd image store.
			dockerLocalDigest := "sha256:7da29d4d35b82e4412a41afd99398c64cc94d58fb5a701c73c684ed22201a14b"
			// Manifest digest created by 'docker push' when *not* using containerd image store.
			dockerDistribDigest := "sha256:f6e9a69f79d3bb745090d8bcd1d17ed24c1993d013d7b5b536fb7d0b61018ad7"

			t.Cleanup(
				func() {
					for _, img := range []string{imageName, registryImage} {
						_, err := localCli.ImageRemove(ctx, img, image.RemoveOptions{PruneChildren: true})
						if !client.IsErrNotFound(err) {
							assert.NoError(t, err)
						}
					}
				},
			)

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

			// Tag and push the image to unregistry.
			require.NoError(
				t, localCli.ImageTag(ctx, imageName, registryImage), "Failed to tag image '%s' as '%s' locally",
				imageName,
				registryImage,
			)
			require.NoError(
				t, pushImage(ctx, localCli, registryImage, image.PushOptions{Platform: &ociPlatform}),
				"Failed to push image '%s' to unregistry", registryImage,
			)

			img, _, err = remoteCli.ImageInspectWithRaw(ctx, imageName)
			require.NoError(t, err, "Pushed image should appear in the remote Docker")
			if localDockerUsesContainerdImageStore {
				assert.Equal(t, platformDigest, img.ID, "Image ID should match platform-specific image digest")
			} else {
				assert.Equal(t, dockerDistribDigest, img.ID, "Image ID should match Docker distribution digest")
			}

			// Remove the image locally before pulling it back.
			for _, img := range []string{imageName, registryImage} {
				_, err = localCli.ImageRemove(ctx, img, image.RemoveOptions{PruneChildren: true})
				require.NoError(t, err, "Failed to remove image '%s' locally", img)
			}

			// Pull the image back from unregistry.
			require.NoError(
				t, pullImage(ctx, localCli, registryImage, image.PullOptions{Platform: platform}),
				"Failed to pull image '%s' from unregistry", registryImage,
			)
			img, _, err = localCli.ImageInspectWithRaw(ctx, registryImage)
			require.NoError(t, err)
			if localDockerUsesContainerdImageStore {
				assert.Equal(t, platformDigest, img.ID, "Pulled image ID should match platform-specific image digest")
			} else {
				assert.Equal(t, dockerLocalDigest, img.ID, "Pulled image ID should match local Docker image digest")
			}

			// Remove the image locally again to test pulling it with arbitrary platform.
			_, err = localCli.ImageRemove(ctx, registryImage, image.RemoveOptions{PruneChildren: true})
			require.NoError(t, err, "Failed to remove image '%s' locally", img)

			// This is a bit weird, but it's the default behavior of the distribution registry.
			require.NoError(
				t, pullImage(ctx, localCli, registryImage, image.PullOptions{Platform: "linux/any-platform"}),
				"Pulling arbitrary platform should pull the existing platform-specific image",
			)

			img, _, err = localCli.ImageInspectWithRaw(ctx, registryImage)
			require.NoError(t, err)
			if localDockerUsesContainerdImageStore {
				assert.Equal(
					t, platformDigest, img.ID, "Arbitrary platform pull should match platform-specific image digest",
				)
			} else {
				assert.Equal(
					t, dockerLocalDigest, img.ID, "Arbitrary platform pull should match local Docker image digest",
				)
			}
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

	// TODO: test pushing an image with digest.
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
