package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinburke/ssh_config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const VERSION = "0.4.0"

var (
	nerdctlPath         = flag.String("nerdctl-path", "nerdctl", "Path to nerdctl binary")
	containerdNamespace = flag.String("namespace", "moby", "Containerd namespace")
	sshKey              = flag.String("i", "", "SSH key path")
	sshPort             = flag.Int("p", 0, "SSH port")
	help                = flag.Bool("h", false, "Show help")
	version             = flag.Bool("version", false, "Show version")
	verbose             = flag.Bool("v", false, "Verbose output")
	pull                = flag.Bool("pull", false, "Pull mode")
	logger              *slog.Logger
)

var containerdSocketPaths = []string{"/run/containerd/containerd.sock", "/var/run/docker/containerd/containerd.sock", "/var/run/containerd/containerd.sock", "/run/docker/containerd/containerd.sock", "/run/snap.docker/containerd/containerd.sock"}
var unregistryImage = func() string {
	if img := os.Getenv("UNREGISTRY_IMAGE"); img != "" {
		return img
	}
	return "infiniflow/unregistry:latest"
}()

func main() {
	flag.Usage = usage
	flag.Parse()

	// Setup logging
	if *verbose {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	// Handle version or help
	if *version {
		fmt.Println("pussh, version", VERSION)
		fmt.Println("unregistry image: ", unregistryImage)
		os.Exit(0)
	}

	if *help || flag.NArg() < 2 {
		usage()
		os.Exit(0)
	}

	image := flag.Arg(0)
	host := flag.Arg(1)

	mode := "push"
	if *pull {
		mode = "pull"
	}
	logger.Info("Starting pussh", "mode", mode, "image", image, "host", host)

	imgName := image
	imgNamespace := *containerdNamespace
	if strings.Contains(image, "::") {
		parts := strings.SplitN(image, "::", 2)
		imgName = parts[1]
		imgNamespace = parts[0]
	}

	// SSH setup
	logger.Info("Setting up SSH", "host", host)
	sshClient := setupSSH(host)
	logger.Info("SSH connected")

	// Remote setup
	remoteNerdctl, remoteNerdctlSudo := checkRemoteNerdctl(sshClient)
	remoteSocket := findRemoteContainerdSocket(sshClient)
	logger.Info("Remote nerdctl", "nerdctl", remoteNerdctl, "sudo", remoteNerdctlSudo != "")
	logger.Info("Remote containerd socket", "socket", remoteSocket)

	if mode == "push" {
		handlePush(sshClient, remoteNerdctl, remoteNerdctlSudo, remoteSocket, imgName, imgNamespace, image, host)
	} else {
		handlePull(sshClient, remoteNerdctl, remoteNerdctlSudo, remoteSocket, imgName, imgNamespace, image, host)
	}
}

func handlePush(sshClient *ssh.Client, remoteNerdctl, remoteNerdctlSudo, remoteSocket, imgName, imgNamespace, image, host string) {
	// Local setup
	localSocket := findLocalContainerdSocket()
	localSudo := checkLocalSudo(localSocket)
	pushNamespace := checkImage(imgName, imgNamespace, localSocket, localSudo)
	logger.Info("Local containerd socket", "socket", localSocket)
	logger.Info("Local sudo needed", "sudo", localSudo != "")
	logger.Info("Image found in namespace", "namespace", pushNamespace)

	// Start unregistry and setup forwarding
	registryPort, unregistryPort, unregistryContainer := startUnregistryWithForwarding(sshClient, remoteNerdctl, remoteNerdctlSudo, remoteSocket, unregistryImage, host)

	// Push local image to unregistry
	pushLocalImageToRegistry(imgName, pushNamespace, registryPort, localSocket, localSudo)

	// Pull from remote
	if remoteNerdctl == "" {
		logger.Error("nerdctl not found on remote host")
		os.Exit(1)
	}
	pullRemoteImageFromRegistry(sshClient, remoteNerdctl, remoteNerdctlSudo, imgName, unregistryPort, imgNamespace)

	// Cleanup
	logger.Info("Cleaning up")
	cleanup(sshClient, remoteNerdctl, remoteNerdctlSudo, unregistryContainer)

	fmt.Printf("Successfully pushed %s to %s\n", image, host)
}

func handlePull(sshClient *ssh.Client, remoteNerdctl, remoteNerdctlSudo, remoteSocket, imgName, imgNamespace, image, host string) {
	// Check remote image
	if remoteNerdctl == "" {
		logger.Error("nerdctl not found on remote host")
		os.Exit(1)
	}
	checkRemoteImage(sshClient, remoteNerdctl, remoteNerdctlSudo, imgName, imgNamespace)

	// Start unregistry and setup forwarding
	registryPort, unregistryPort, unregistryContainer := startUnregistryWithForwarding(sshClient, remoteNerdctl, remoteNerdctlSudo, remoteSocket, unregistryImage, host)

	// Push remote image to unregistry
	pushRemoteImageToRegistry(sshClient, remoteNerdctl, remoteNerdctlSudo, imgName, unregistryPort, imgNamespace)

	// Pull to local
	pullLocalImageFromRegistry(imgName, registryPort, imgNamespace)

	// Cleanup
	logger.Info("Cleaning up")
	cleanup(sshClient, remoteNerdctl, remoteNerdctlSudo, unregistryContainer)

	fmt.Printf("Successfully pulled %s from %s\n", image, host)
}

func startUnregistryWithForwarding(sshClient *ssh.Client, remoteNerdctl, remoteNerdctlSudo, remoteSocket, unregistryImage, host string) (int, int, string) {
	logger.Info("Starting unregistry")
	unregistryPort, unregistryContainer := startUnregistry(sshClient, remoteNerdctl, remoteNerdctlSudo, remoteSocket, unregistryImage, host)
	logger.Info("Unregistry started", "port", unregistryPort, "container", unregistryContainer)

	logger.Info("Setting up port forwarding")
	localPort := forwardPort(sshClient, unregistryPort)
	logger.Info("Port forwarding set up", "local_port", localPort)

	return localPort, unregistryPort, unregistryContainer
}

func pushLocalImageToRegistry(imgName, pushNamespace string, registryPort int, localSocket, localSudo string) {
	registryImage := fmt.Sprintf("localhost:%d/%s", registryPort, imgName)
	logger.Info("Tagging image", "image", registryImage)
	tagImage(imgName, registryImage, pushNamespace, localSocket, localSudo)
	logger.Info("Pushing image")
	pushImage(registryImage, pushNamespace, localSocket, localSudo)
	logger.Info("Cleaning up temporary image tag", "image", registryImage)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s rmi %s", localSudo, *nerdctlPath, localSocket, pushNamespace, registryImage))
	cmd.Run() // ignore error
}

func pullRemoteImageFromRegistry(sshClient *ssh.Client, remoteNerdctl, remoteNerdctlSudo, imgName string, unregistryPort int, imgNamespace string) {
	registryImage := fmt.Sprintf("127.0.0.1:%d/%s", unregistryPort, imgName)
	logger.Info("Pulling with nerdctl")
	pullWithNerdctl(sshClient, remoteNerdctl, remoteNerdctlSudo, registryImage, imgName, imgNamespace)
}

func pushRemoteImageToRegistry(sshClient *ssh.Client, remoteNerdctl, remoteNerdctlSudo, imgName string, unregistryPort int, imgNamespace string) {
	registryImage := fmt.Sprintf("127.0.0.1:%d/%s", unregistryPort, imgName)
	logger.Info("Tagging remote image", "image", registryImage)
	tagRemoteImage(sshClient, remoteNerdctl, remoteNerdctlSudo, imgName, registryImage, imgNamespace)
	logger.Info("Pushing remote image")
	pushRemoteImage(sshClient, remoteNerdctl, remoteNerdctlSudo, registryImage, imgNamespace)
}

func pullLocalImageFromRegistry(imgName string, registryPort int, imgNamespace string) {
	registryImage := fmt.Sprintf("localhost:%d/%s", registryPort, imgName)
	logger.Info("Pulling with nerdctl")
	pullLocalImage(registryImage, imgName, imgNamespace)
}

func findLocalContainerdSocket() string {
	for _, p := range containerdSocketPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	logger.Error("Local containerd socket not found")
	os.Exit(1)
	return ""
}

func findRemoteContainerdSocket(client *ssh.Client) string {
	for _, p := range containerdSocketPaths {
		session, _ := client.NewSession()
		err := session.Run(fmt.Sprintf("test -S %s", p))
		session.Close()
		if err == nil {
			return p
		}
	}
	logger.Error("Remote containerd socket not found")
	os.Exit(1)
	return ""
}

func checkLocalSudo(socket string) string {
	logger.Debug("Checking local sudo for nerdctl")
	cmd := exec.Command(*nerdctlPath, "--address", socket, "namespace", "ls")
	if err := cmd.Run(); err != nil {
		logger.Debug("nerdctl namespace ls failed without sudo, trying with sudo")
		cmd = exec.Command("sudo", "-n", *nerdctlPath, "--address", socket, "namespace", "ls")
		if err := cmd.Run(); err != nil {
			logger.Error("Failed to run nerdctl locally")
			os.Exit(1)
		}
		return "sudo -n"
	}
	logger.Debug("nerdctl works without sudo")
	return ""
}

func checkImage(name, namespace, socket, sudo string) string {
	logger.Debug("Checking image", "name", name, "namespace", namespace)
	cmdStr := fmt.Sprintf("%s %s --address %s namespace ls | tail -n +2 | awk '{print $1}'", sudo, *nerdctlPath, socket)
	logger.Debug("Running command", "cmd", cmdStr)
	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to list namespaces", "error", err)
		os.Exit(1)
	}
	logger.Debug("Namespaces output", "output", string(output))
	namespaces := strings.Split(strings.TrimSpace(string(output)), "\n")
	logger.Debug("Namespaces", "namespaces", namespaces)
	start := -1
	for i, ns := range namespaces {
		if ns == namespace {
			start = i
			break
		}
	}
	if start == -1 {
		logger.Error("Specified namespace not found", "namespace", namespace)
		os.Exit(1)
	}
	logger.Debug("Starting from namespace index", "index", start)
	for i := start; i < len(namespaces); i++ {
		ns := namespaces[i]
		cmdStr = fmt.Sprintf("%s %s --address %s --namespace %s image inspect %s", sudo, *nerdctlPath, socket, ns, name)
		logger.Debug("Checking namespace", "namespace", ns, "cmd", cmdStr)
		cmd = exec.Command("sh", "-c", cmdStr)
		if err := cmd.Run(); err == nil {
			logger.Debug("Image found in namespace", "namespace", ns)
			return ns
		}
	}
	for i := 0; i < start; i++ {
		ns := namespaces[i]
		cmdStr = fmt.Sprintf("%s %s --address %s --namespace %s image inspect %s", sudo, *nerdctlPath, socket, ns, name)
		logger.Debug("Checking namespace", "namespace", ns, "cmd", cmdStr)
		cmd = exec.Command("sh", "-c", cmdStr)
		if err := cmd.Run(); err == nil {
			logger.Debug("Image found in namespace", "namespace", ns)
			return ns
		}
	}
	logger.Error("Image not found in any namespace", "image", name)
	os.Exit(1)
	return ""
}

func setupSSH(host string) *ssh.Client {
	var user, h string
	var port int = 0
	var identityFiles []string

	// Parse host string for user and host
	if strings.Contains(host, "@") {
		parts := strings.SplitN(host, "@", 2)
		user = parts[0]
		h = parts[1]
	} else {
		h = host
	}

	// Parse host string for port (host:port format)
	if strings.Contains(h, ":") {
		parts := strings.SplitN(h, ":", 2)
		h = parts[0]
		_, _ = fmt.Sscanf(parts[1], "%d", &port)
	}

	// Apply priority: command line > SSH config > default
	if *sshPort != 0 {
		port = *sshPort
	}

	// Load SSH config (only if command line port not specified)
	home, _ := os.UserHomeDir()
	if *sshPort == 0 {
		configPath := filepath.Join(home, ".ssh", "config")
		sshConfigData, err := os.ReadFile(configPath)
		var userMismatch bool
		if err == nil {
			cfg, err := ssh_config.Decode(strings.NewReader(string(sshConfigData)))
			if err == nil {
				if configUser, _ := cfg.Get(h, "User"); configUser != "" {
					if user == "" {
						user = configUser
					} else if user != configUser {
						userMismatch = true
					}
				}
				if !userMismatch {
					if port == 0 {
						if configPort, _ := cfg.Get(h, "Port"); configPort != "" {
							_, _ = fmt.Sscanf(configPort, "%d", &port)
						}
					}
					if hostname, _ := cfg.Get(h, "Hostname"); hostname != "" {
						h = hostname
					}
					// Get IdentityFile from SSH config
					if configIdentityFile, _ := cfg.Get(h, "IdentityFile"); configIdentityFile != "" {
						// Expand ~ to home directory
						if strings.HasPrefix(configIdentityFile, "~") {
							configIdentityFile = filepath.Join(home, configIdentityFile[1:])
						}
						identityFiles = append(identityFiles, configIdentityFile)
					}
				}
			}
		}
	}
	if port == 0 {
		port = 22
	}
	logger.Debug("SSH config loaded", "h", h, "user", user, "port", port)
	var addr string
	if port != 22 {
		addr = fmt.Sprintf("%s:%d", h, port)
	} else {
		addr = h + ":22"
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// Load SSH keys in priority order: command line > SSH config > default files
	var keyPaths []string

	// 1. Command line specified key
	if *sshKey != "" {
		keyPaths = append(keyPaths, *sshKey)
	}

	// 2. SSH config IdentityFile
	keyPaths = append(keyPaths, identityFiles...)

	// 3. Default identity files
	if len(keyPaths) == 0 || (*sshKey == "" && len(identityFiles) == 0) {
		logger.Debug("Loading default keys")
		defaultKeys := []string{
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
		keyPaths = append(keyPaths, defaultKeys...)
	}

	// Load and parse keys
	for _, keyPath := range keyPaths {
		if _, err := os.Stat(keyPath); err == nil {
			logger.Debug("Loading key", "file", keyPath)
			key, err := os.ReadFile(keyPath)
			if err != nil {
				logger.Debug("Failed to read key", "file", keyPath, "error", err)
				continue
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				logger.Debug("Failed to parse key", "file", keyPath, "error", err)
				continue
			}
			config.Auth = append(config.Auth, ssh.PublicKeys(signer))
		}
	}

	if len(config.Auth) == 0 {
		logger.Error("No SSH authentication methods available")
		os.Exit(1)
	}
	logger.Debug("Auth methods loaded", "count", len(config.Auth))
	logger.Debug("Dialing SSH", "user", user, "addr", addr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		logger.Error("Failed to dial SSH", "error", err)
		os.Exit(1)
	}
	return client
}

func usage() {
	fmt.Print(`pussh - Push/pull containerd images to/from remote hosts via SSH without external registries.

DESIGN:
This program rewrites the original bash script docker-pussh in Go for better maintainability and error handling.

It performs the following steps:
1. Local checks: Find the local containerd socket, check if sudo is needed for nerdctl, and locate the specified image in containerd namespaces.
2. SSH setup: Establish an SSH connection to the remote host using key-based or agent authentication.
3. Remote checks: Detect available nerdctl binaries on the remote host, including sudo requirements.
4. Start unregistry: Launch a temporary unregistry container on the remote host to act as a local registry.
5. Port forwarding: Set up SSH port forwarding to tunnel traffic to the unregistry container.
6. Push image: Tag the local image with the registry address and push it to the forwarded port.
7. Pull image: On the remote host, pull the image from the registry using nerdctl, tag it appropriately, and clean up.
8. Cleanup: Stop and remove the temporary unregistry container.

Logging: Uses slog for structured logging. The -v flag enables debug-level logs for troubleshooting.

PREREQUISITES:
- Both local and remote hosts must have containerd and nerdctl installed
- SSH key-based authentication must be configured (password authentication not supported)
- SSH connection must allow port forwarding
- User must have permission to run nerdctl commands (may require sudo)

USAGE: pussh [OPTIONS] IMAGE HOST

Transfer container images between local and remote hosts via SSH without an external registry.

OPTIONS:
  -h, --help                Show this help message.
  -i string                 Path to SSH private key.
  -p int                    SSH port (default 22).
  --pull                    Pull mode (download from remote to local).

ENVIRONMENT VARIABLES:
  UNREGISTRY_AIR_GAPPED     Enable air-gapped mode: transfer unregistry image from local if not available on remote (default: 0)
  UNREGISTRY_IMAGE          Specify custom unregistry image (default: infiniflow/unregistry:latest)

EXAMPLES:
  # Push mode (default)
  pussh ubuntu:24.04 inf96
  pussh k8s.io::ubuntu:24.04 inf96

  # Pull mode
  pussh --pull k8s.io::ubuntu:24.04 inf96
  pussh --pull ubuntu:24.04 inf96

  # Air-gapped mode (transfer unregistry image if needed)
  UNREGISTRY_AIR_GAPPED=1 pussh ubuntu:24.04 inf96
  UNREGISTRY_AIR_GAPPED=1 UNREGISTRY_IMAGE=my-registry.com/unregistry:latest pussh ubuntu:24.04 inf96
`)
}

func checkRemoteImage(client *ssh.Client, nerdctl, nerdctlSudo, image, namespace string) {
	cmd := fmt.Sprintf("%s %s --namespace %s image inspect %s", nerdctlSudo, nerdctl, namespace, image)
	session, _ := client.NewSession()
	err := session.Run(cmd)
	session.Close()
	if err != nil {
		logger.Error("Remote image not found", "image", image)
		os.Exit(1)
	}
}

func tagRemoteImage(client *ssh.Client, nerdctl, nerdctlSudo, name, registryImage, namespace string) {
	cmd := fmt.Sprintf("%s %s --namespace %s tag %s %s", nerdctlSudo, nerdctl, namespace, name, registryImage)
	session, _ := client.NewSession()
	err := session.Run(cmd)
	session.Close()
	if err != nil {
		logger.Error("Failed to tag remote image", "error", err)
		os.Exit(1)
	}
}

func pushRemoteImage(client *ssh.Client, nerdctl, nerdctlSudo, registryImage, namespace string) {
	cmd := fmt.Sprintf("%s %s --namespace %s push %s", nerdctlSudo, nerdctl, namespace, registryImage)
	session, _ := client.NewSession()
	err := session.Run(cmd)
	session.Close()
	if err != nil {
		logger.Error("Failed to push remote image", "error", err)
		os.Exit(1)
	}
}

func pullLocalImage(registryImage, name, namespace string) {
	localSocket := findLocalContainerdSocket()
	localSudo := checkLocalSudo(localSocket)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s pull %s", localSudo, *nerdctlPath, localSocket, namespace, registryImage))
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to pull local image", "error", err)
		os.Exit(1)
	}
	// Tag
	cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s tag %s %s", localSudo, *nerdctlPath, localSocket, namespace, registryImage, name))
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to tag local image", "error", err)
		os.Exit(1)
	}
	// Rmi
	cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s rmi %s", localSudo, *nerdctlPath, localSocket, namespace, registryImage))
	cmd.Run() // ignore error
}

func checkRemoteNerdctl(client *ssh.Client) (string, string) {
	paths := []string{"nerdctl", "/usr/bin/nerdctl", "/usr/local/bin/nerdctl"}
	for _, p := range paths {
		session, _ := client.NewSession()
		err := session.Run(fmt.Sprintf("test -x %s", p))
		session.Close()
		if err == nil {
			session, _ := client.NewSession()
			err = session.Run(fmt.Sprintf("%s image ls", p))
			session.Close()
			if err != nil {
				session, _ := client.NewSession()
				err = session.Run(fmt.Sprintf("sudo -n %s image ls", p))
				session.Close()
				if err == nil {
					return p, "sudo -n"
				}
			} else {
				return p, ""
			}
		}
	}
	return "", ""
}

func startUnregistry(client *ssh.Client, nerdctl, nerdctlSudo, socket, image, host string) (int, string) {
	port := 55000 + int(time.Now().UnixNano()%10000)
	container := fmt.Sprintf("unregistry-pussh-%d", time.Now().Unix())

	// Clean up any existing pussh containers to avoid conflicts
	logger.Info("Cleaning up existing pussh containers")
	cleanupCmd := fmt.Sprintf("%s %s ps -aq --filter name=unregistry-pussh- | xargs %s %s rm -f", nerdctlSudo, nerdctl, nerdctlSudo, nerdctl)
	cleanupSession, _ := client.NewSession()
	cleanupSession.Run(cleanupCmd)
	cleanupSession.Close()

	// Check if unregistry image is available on remote
	imageAvailable := checkRemoteImageExists(client, nerdctl, nerdctlSudo, image)

	if !imageAvailable {
		if os.Getenv("UNREGISTRY_AIR_GAPPED") != "" {
			// Air-gapped mode: transfer image from local
			logger.Info("Unregistry image not found on remote, transferring from local (air-gapped mode)", "image", image)
			transferUnregistryImage(client, nerdctl, nerdctlSudo, image, host)
		} else {
			// Try to pull image on remote
			logger.Info("Unregistry image not found on remote, attempting to pull", "image", image)
			pullCmd := fmt.Sprintf("%s %s pull %s", nerdctlSudo, nerdctl, image)
			session, _ := client.NewSession()
			err := session.Run(pullCmd)
			session.Close()
			if err != nil {
				logger.Error("Failed to pull unregistry image on remote, try setting UNREGISTRY_AIR_GAPPED=1 to transfer from local", "error", err, "image", image)
				os.Exit(1)
			}
			logger.Info("Successfully pulled unregistry image on remote", "image", image)
		}
	}

	cmd := fmt.Sprintf("%s %s run -d --name %s --net host -v %s:/run/containerd/containerd.sock %s -a :%d", nerdctlSudo, nerdctl, container, socket, image, port)
	session, _ := client.NewSession()
	err := session.Run(cmd)
	session.Close()
	if err != nil {
		logger.Error("Failed to start unregistry", "error", err)
		os.Exit(1)
	}
	return port, container
}

func forwardPort(client *ssh.Client, remotePort int) int {
	localPort := 55000 + int(time.Now().UnixNano()%10000)
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		logger.Error("Failed to listen on local port", "error", err)
		os.Exit(1)
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				remoteConn, err := client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", remotePort))
				if err != nil {
					conn.Close()
					return
				}
				go io.Copy(conn, remoteConn)
				io.Copy(remoteConn, conn)
			}()
		}
	}()
	return localPort
}

func tagImage(name, registryImage, namespace, socket, sudo string) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s tag %s %s", sudo, *nerdctlPath, socket, namespace, name, registryImage))
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to tag image", "error", err)
		os.Exit(1)
	}
}

func pushImage(registryImage, namespace, socket, sudo string) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s --namespace %s push %s", sudo, *nerdctlPath, socket, namespace, registryImage))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to push image", "error", err, "output", string(output))
		os.Exit(1)
	}
}

func pullWithNerdctl(client *ssh.Client, nerdctl, sudo, registryImage, name, namespace string) {
	var sessTag4, sessRmi4 *ssh.Session
	cmd := fmt.Sprintf("%s %s --namespace %s pull %s", sudo, nerdctl, namespace, registryImage)
	logger.Debug("Pull command", "cmd", cmd)
	session, _ := client.NewSession()
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	err := session.Run(cmd)
	session.Close()
	if err != nil {
		logger.Error("Failed to pull image with nerdctl", "error", err, "stdout", stdout.String(), "stderr", stderr.String())
		os.Exit(1)
	}
	cmd = fmt.Sprintf("%s %s --namespace %s tag %s %s", sudo, nerdctl, namespace, registryImage, name)
	logger.Debug("Tag command", "cmd", cmd)
	sessTag4, _ = client.NewSession()
	sessTag4.Run(cmd)
	sessTag4.Close()
	cmd = fmt.Sprintf("%s %s --namespace %s rmi %s", sudo, nerdctl, namespace, registryImage)
	logger.Debug("Rmi command", "cmd", cmd)
	sessRmi4, _ = client.NewSession()
	err = sessRmi4.Run(cmd)
	sessRmi4.Close()
	if err != nil {
		logger.Warn("Failed to remove temporary registry image", "error", err, "image", registryImage)
	}
}

func cleanup(client *ssh.Client, nerdctl, nerdctlSudo, container string) {
	cmd := fmt.Sprintf("%s %s rm -f %s", nerdctlSudo, nerdctl, container)
	session, _ := client.NewSession()
	session.Run(cmd)
	session.Close()
}

func checkRemoteImageExists(client *ssh.Client, nerdctl, nerdctlSudo, image string) bool {
	cmd := fmt.Sprintf("%s %s image inspect %s", nerdctlSudo, nerdctl, image)
	session, _ := client.NewSession()
	err := session.Run(cmd)
	session.Close()
	return err == nil
}

func checkLocalImageExists(image, socket, sudo string) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s image inspect %s", sudo, *nerdctlPath, socket, image))
	return cmd.Run() == nil
}

func transferUnregistryImage(client *ssh.Client, nerdctl, nerdctlSudo, image, remoteHost string) {
	localSocket := findLocalContainerdSocket()
	localSudo := checkLocalSudo(localSocket)

	// Ensure the unregistry image exists locally
	if !checkLocalImageExists(image, localSocket, localSudo) {
		logger.Info("Pulling unregistry image locally", "image", image)
		pullCmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s pull %s", localSudo, *nerdctlPath, localSocket, image))
		if err := pullCmd.Run(); err != nil {
			logger.Error("Failed to pull unregistry image locally", "error", err, "image", image)
			logger.Error("Make sure the image is available locally or proxy is configured correctly")
			os.Exit(1)
		}
	}

	// Export image from local
	logger.Info("Exporting unregistry image from local")
	tempFile := fmt.Sprintf("/tmp/unregistry-%d.tar", time.Now().Unix())
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s %s --address %s save -o %s %s", localSudo, *nerdctlPath, localSocket, tempFile, image))
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to export unregistry image locally", "error", err)
		os.Exit(1)
	}
	defer os.Remove(tempFile)

	// Transfer tar file to remote via SFTP
	logger.Info("Transferring unregistry image to remote via SFTP")
	remoteTempFile := fmt.Sprintf("/tmp/unregistry-remote-%d.tar", time.Now().Unix())

	// Create SFTP client
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		logger.Error("Failed to create SFTP client", "error", err)
		os.Exit(1)
	}
	defer sftpClient.Close()

	// Open local file
	localFile, err := os.Open(tempFile)
	if err != nil {
		logger.Error("Failed to open local image file", "error", err)
		os.Exit(1)
	}
	defer localFile.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create(remoteTempFile)
	if err != nil {
		logger.Error("Failed to create remote file", "error", err)
		os.Exit(1)
	}
	defer remoteFile.Close()

	// Copy file content
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		logger.Error("Failed to transfer file via SFTP", "error", err)
		os.Exit(1)
	}

	logger.Info("Successfully transferred image file to remote", "remote_file", remoteTempFile)

	// Load image on remote
	logger.Info("Loading unregistry image on remote")
	loadCmd := fmt.Sprintf("%s %s load -i %s", nerdctlSudo, nerdctl, remoteTempFile)
	session, _ := client.NewSession()
	err = session.Run(loadCmd)
	session.Close()
	if err != nil {
		logger.Error("Failed to load unregistry image on remote", "error", err)
		os.Exit(1)
	}

	// Clean up remote temp file
	cleanupCmd := fmt.Sprintf("rm -f %s", remoteTempFile)
	session, _ = client.NewSession()
	session.Run(cleanupCmd)
	session.Close()
}
