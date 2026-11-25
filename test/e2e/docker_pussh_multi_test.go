package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// unregistryImageRepo is the repository of the unregistry image that docker-pussh starts on the remote host.
	unregistryImageRepo = "ghcr.io/psviderski/unregistry"
	// dockerPusshTimeout bounds one docker-pussh run so a hung run fails its test instead of stalling the package.
	dockerPusshTimeout = 10 * time.Minute
)

// TestDockerPusshMultipleImages tests pushing several images to a remote host in a single docker-pussh run.
// All images must reach the remote Docker, and exactly one unregistry container must serve all of them.
//
// The image tags below belong to this test alone. The tests run in parallel and remove the tags they pulled when they
// finish, so a tag shared with another test would let one cleanup delete an image the other is still pushing.
func TestDockerPusshMultipleImages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	localCli := newDockerClient(t, client.FromEnv)

	images := []string{"busybox:1.36", "alpine:3.20"}
	pullImagesForTest(ctx, t, localCli, images, "linux/amd64")

	tests := []struct {
		name            string
		registryPort    int
		containerdStore bool
	}{
		{
			name:            "native image store",
			registryPort:    50003,
			containerdStore: false,
		},
		{
			name:            "containerd image store",
			registryPort:    50004,
			containerdStore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dockerPort, sshPort := runUnregistryDinD(t, tt.registryPort, tt.containerdStore)
			remoteCli := newDockerClient(t, client.WithHost("tcp://localhost:"+dockerPort))

			for _, imageName := range images {
				_, _, err := remoteCli.ImageInspectWithRaw(ctx, imageName)
				require.Error(t, err, "Image %s should not exist on remote before pushing", imageName)
			}

			// Watch the remote daemon before pushing so that every container docker-pussh starts is recorded.
			watcher := watchUnregistryContainers(t, remoteCli)

			args := []string{"-i", dockerPusshSSHKey(t), "--no-host-key-check"}
			args = append(args, images...)
			args = append(args, fmt.Sprintf("root@localhost:%s", sshPort))
			_, err := runDockerPussh(t, args...)
			require.NoError(t, err, "Failed to run docker-pussh")

			assert.Len(
				t, watcher.waitCreated(t), 1,
				"docker-pussh should start a single unregistry container for all images",
			)

			for _, imageName := range images {
				_, _, err = remoteCli.ImageInspectWithRaw(ctx, imageName)
				require.NoError(t, err, "Pushed image %s should appear in the remote Docker", imageName)
			}
		})
	}
}

// TestDockerPusshMultipleImagesPlatform pushes several images with --platform in one run and checks that
// the remote Docker received the requested architecture. Holding more than one platform of an image index
// locally requires the containerd image store, so the test is skipped otherwise.
//
// It uses image tags no other test uses, see TestDockerPusshMultipleImages.
func TestDockerPusshMultipleImagesPlatform(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	localCli := newDockerClient(t, client.FromEnv)

	info, err := localCli.Info(ctx)
	require.NoError(t, err)
	if !hasContainerdStore(info.DriverStatus) {
		t.Skip("local Docker does not use the containerd image store")
	}

	images := []string{"busybox:1.37", "alpine:3.21"}
	const platform = "linux/arm64"
	const wantArch = "arm64"
	pullImagesForTest(ctx, t, localCli, images, platform)

	dockerPort, sshPort := runUnregistryDinD(t, 50005, true)
	remoteCli := newDockerClient(t, client.WithHost("tcp://localhost:"+dockerPort))
	watcher := watchUnregistryContainers(t, remoteCli)

	args := []string{"--platform", platform, "-i", dockerPusshSSHKey(t), "--no-host-key-check"}
	args = append(args, images...)
	args = append(args, fmt.Sprintf("root@localhost:%s", sshPort))
	_, err = runDockerPussh(t, args...)
	require.NoError(t, err, "Failed to run docker-pussh")

	assert.Len(
		t, watcher.waitCreated(t), 1,
		"docker-pussh should start a single unregistry container for all images",
	)

	for _, imageName := range images {
		inspect, _, err := remoteCli.ImageInspectWithRaw(ctx, imageName)
		require.NoError(t, err, "Pushed image %s should appear in the remote Docker", imageName)
		assert.Equal(t, wantArch, inspect.Architecture, "Remote image %s should be %s", imageName, wantArch)
	}
}

// TestDockerPusshMissingImage checks that images are validated in the local Docker before docker-pussh connects to the
// host. A mistyped image, or an extra argument that is taken for an image, must fail the run before anything is
// pushed instead of after the images listed before it have already been sent.
func TestDockerPusshMissingImage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	localCli := newDockerClient(t, client.FromEnv)

	// An image that exists locally, under a tag only this test uses.
	const sourceImage = "hello-world:latest"
	const presentImage = "pussh-e2e-missing/present:latest"
	require.NoError(t, pullImage(ctx, localCli, sourceImage, image.PullOptions{}), "Failed to pull %s", sourceImage)
	require.NoError(t, localCli.ImageTag(ctx, sourceImage, presentImage))
	t.Cleanup(func() {
		_, err := localCli.ImageRemove(ctx, presentImage, image.RemoveOptions{})
		if !client.IsErrNotFound(err) {
			assert.NoError(t, err)
		}
	})

	// The host is unreachable on purpose: a run that got as far as connecting would fail with a different error.
	const unreachableHost = "root@127.0.0.1:1"
	tests := []struct {
		name        string
		args        []string
		wantMissing string
	}{
		{
			name:        "mistyped image",
			args:        []string{presentImage, "no-such/image:tag", unreachableHost},
			wantMissing: "no-such/image:tag",
		},
		{
			name:        "two hosts",
			args:        []string{presentImage, unreachableHost, "root@127.0.0.1:2"},
			wantMissing: unreachableHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := runDockerPussh(t, tt.args...)
			require.Error(t, err, "docker-pussh should fail")
			assert.Contains(t, output, "Image not found in the local Docker: "+tt.wantMissing)
			assert.NotContains(t, output, "Connecting to", "docker-pussh should fail before connecting to the host")
		})
	}
}

// newDockerClient creates a Docker client that is closed when the test ends.
func newDockerClient(t *testing.T, opt client.Opt) *client.Client {
	t.Helper()

	cli, err := client.NewClientWithOpts(opt, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, cli.Close()) })

	return cli
}

// pullImagesForTest pulls the images for the platform and removes them when the test ends.
func pullImagesForTest(ctx context.Context, t *testing.T, cli *client.Client, images []string, platform string) {
	t.Helper()

	t.Cleanup(func() {
		for _, imageName := range images {
			_, err := cli.ImageRemove(ctx, imageName, image.RemoveOptions{PruneChildren: true})
			if !client.IsErrNotFound(err) {
				assert.NoError(t, err)
			}
		}
	})

	for _, imageName := range images {
		require.NoError(
			t, pullImage(ctx, cli, imageName, image.PullOptions{Platform: platform}),
			"Failed to pull image '%s' for %s locally", imageName, platform,
		)
	}
}

// dockerPusshSSHKey returns the path to the private key of the test SSH server.
func dockerPusshSSHKey(t *testing.T) string {
	t.Helper()

	sshKeyPath := filepath.Join(projectRoot(), "test/e2e/ssh/test_key")
	// Change permission of the ssh key to 0600 to avoid SSH WARNING: UNPROTECTED PRIVATE KEY FILE!
	require.NoError(t, os.Chmod(sshKeyPath, 0600), "Failed to change permission of SSH key")

	return sshKeyPath
}

// runDockerPussh runs docker-pussh with the arguments and returns its combined output and the run error.
func runDockerPussh(t *testing.T, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerPusshTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, filepath.Join(projectRoot(), "docker-pussh"), args...)
	t.Logf("Running docker-pussh command: %s", cmd.String())

	output, err := cmd.CombinedOutput()
	t.Logf("docker-pussh output:\n%s", string(output))

	return string(output), err
}

// hasContainerdStore reports whether the Docker daemon uses the containerd image store, which is
// advertised by the snapshotter driver-type in `docker info`.
func hasContainerdStore(driverStatus [][2]string) bool {
	for _, status := range driverStatus {
		if status[0] == "driver-type" && strings.HasPrefix(status[1], "io.containerd.snapshotter") {
			return true
		}
	}
	return false
}

// unregistryContainerWatcher records the unregistry containers that are created and removed on a Docker daemon.
// Unlike counting log lines it observes what the daemon really ran, so a run that started one container per image
// is caught even if the output looked the same.
type unregistryContainerWatcher struct {
	mu        sync.Mutex
	created   []string
	destroyed int
	// streamErr is the error that ended the event stream, if any.
	streamErr error
}

// watchUnregistryContainers starts recording unregistry container events of the daemon until the test ends.
// Start it before running docker-pussh.
func watchUnregistryContainers(t *testing.T, cli *client.Client) *unregistryContainerWatcher {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	w := &unregistryContainerWatcher{}
	msgs, errs := cli.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(filters.Arg("type", string(events.ContainerEventType))),
	})
	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				w.record(msg)
			case err := <-errs:
				w.setStreamErr(err)
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return w
}

func (w *unregistryContainerWatcher) setStreamErr(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.streamErr = err
}

func (w *unregistryContainerWatcher) record(msg events.Message) {
	if !strings.HasPrefix(msg.Actor.Attributes["image"], unregistryImageRepo) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	switch string(msg.Action) {
	case "create":
		w.created = append(w.created, msg.Actor.Attributes["name"])
	case "destroy":
		w.destroyed++
	}
}

// waitCreated waits until every unregistry container that was created has been removed, which docker-pussh does before
// it exits, and returns the names of the created containers. It fails the test if no container was seen at all, so a
// missed event cannot make a check on the number of containers pass vacuously.
func (w *unregistryContainerWatcher) waitCreated(t *testing.T) []string {
	t.Helper()

	seen := assert.Eventually(t, func() bool {
		w.mu.Lock()
		defer w.mu.Unlock()
		return len(w.created) > 0 && w.destroyed >= len(w.created)
	}, 15*time.Second, 100*time.Millisecond, "Expected to see an unregistry container created and removed")
	if !seen {
		w.mu.Lock()
		streamErr := w.streamErr
		w.mu.Unlock()
		require.NoError(t, streamErr, "The Docker event stream ended with an error")
		t.FailNow()
	}

	// Events arrive in order, so this short grace period lets the events of a second container trail the first one.
	time.Sleep(time.Second)

	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]string(nil), w.created...)
}
