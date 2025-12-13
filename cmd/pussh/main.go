package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const VERSION = "0.4.1"

var (
	startTime      = time.Now()
	help           = flag.Bool("h", false, "Show help")
	version        = flag.Bool("version", false, "Show version")
	verbose        = flag.Bool("verbose", false, "Verbose output")
	pull           = flag.Bool("pull", false, "Pull mode")
	sshKey         = flag.String("i", "", "Path to SSH private key")
	noHostKeyCheck = flag.Bool("no-host-key-check", false, "Skip SSH host key checking")
	platform       = flag.String("platform", "", "Push a specific platform for multi-platform images")
	cachedSSHArgs  []string
	imgName        string
	imgNamespace   string
	remoteHost     string
	useDocker      bool
	remoteTool     string
	remoteToolSudo string
	remoteSocket   string
	localTool      string
	localToolSudo  string
	localSocket    string
	sshTunnel      *exec.Cmd
	unregContainer string
)

var containerdSocketPaths = []string{
	"/run/containerd/containerd.sock",
	"/var/run/containerd/containerd.sock",
	"/run/docker/containerd/containerd.sock",
	"/var/run/docker/containerd/containerd.sock",
	"/run/snap.docker/containerd/containerd.sock",
}

var toolDirectories = []string{
	".",
	".local/bin/",
	"/usr/local/bin/",
	"/usr/bin/",
	"/usr/sbin/",
	"/opt/docker/bin/",
	"/snap/bin/",
	"/storage/.docker/bin",
}

var unregImage = func() string {
	if img := os.Getenv("UNREGISTRY_IMAGE"); img != "" {
		return img
	}
	return "ghcr.io/psviderski/unregistry:0.4.1"
}()

// --- SSH Helpers ---

func buildSSHArgs() []string {
	args := []string{}
	if *sshKey != "" {
		args = append(args, "-i", *sshKey)
	}
	if *noHostKeyCheck {
		args = append(args, "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	}
	args = append(args, remoteHost)
	return args
}

// --- Logging Helpers ---

func logPrintf(level, format string, v ...any) {
	elapsed := int(time.Since(startTime).Seconds())
	prefix := fmt.Sprintf("%-4s[%04d] ", level, elapsed)
	fmt.Fprintf(os.Stderr, prefix+format+"\n", v...)
}

func info(format string, v ...any) { logPrintf("INFO", format, v...) }
func debug(format string, v ...any) {
	if *verbose {
		logPrintf("DEBU", format, v...)
	}
}

// --- Path and Socket Helpers ---

func findContainerdSocket(isRemote bool) (string, string) {
	for _, p := range containerdSocketPaths {
		testCmd := fmt.Sprintf("test -S %s", p)
		debug("Testing socket path: %s", p)
		if isRemote {
			sshArgs := make([]string, len(cachedSSHArgs))
			copy(sshArgs, cachedSSHArgs)
			sshArgs = append(sshArgs, testCmd)
			if exec.Command("ssh", sshArgs...).Run() == nil {
				return p, ""
			}
			sshArgsSudo := make([]string, len(cachedSSHArgs))
			copy(sshArgsSudo, cachedSSHArgs)
			sshArgsSudo = append(sshArgsSudo, "sudo -n "+testCmd)
			if exec.Command("ssh", sshArgsSudo...).Run() == nil {
				return p, "sudo -n"
			}
		} else {
			if exec.Command("sh", "-c", testCmd).Run() == nil {
				return p, ""
			}
			if exec.Command("sh", "-c", "sudo -n "+testCmd).Run() == nil {
				return p, "sudo -n"
			}
		}
	}
	return "", ""
}

// --- Command String Builders ---

func buildToolCmd(isRemote bool, ns string, args ...string) (string, []string) {
	var parts []string
	var tool, toolSudo, socket string
	if isRemote {
		tool, toolSudo, socket = remoteTool, remoteToolSudo, remoteSocket
	} else {
		tool, toolSudo, socket = localTool, localToolSudo, localSocket
	}
	if toolSudo != "" {
		parts = append(parts, toolSudo)
	}
	parts = append(parts, tool)
	if !useDocker {
		if socket != "" {
			parts = append(parts, "--address", socket)
		}
		if ns != "" {
			parts = append(parts, "--namespace", ns)
		}
	}
	parts = append(parts, args...)
	shellCmd := strings.Join(parts, " ")
	var cmd string
	var args2 []string
	if isRemote {
		cmd = "ssh"
		args2 = make([]string, len(cachedSSHArgs))
		copy(args2, cachedSSHArgs)
		args2 = append(args2, shellCmd)
	} else {
		cmd = "bash"
		args2 = []string{"-c", shellCmd}
	}
	debug("Executing: %s %q", cmd, args2)
	return cmd, args2
}

// --- Logic Helpers ---

func runCmdWithLiveOutput(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func checkImageExists(isRemote bool, image string) bool {
	cmd, args := buildToolCmd(isRemote, "", "image", "inspect", image)
	return exec.Command(cmd, args...).Run() == nil
}

func listNamespaces(isRemote bool) ([]string, error) {
	if useDocker {
		return []string{"default"}, nil
	}
	cmd, args := buildToolCmd(isRemote, "", "namespace", "ls")
	output, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var namespaces []string
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			namespaces = append(namespaces, fields[0])
		}
	}
	return namespaces, nil
}

func findImage(isRemote bool, name, preferNamespace string) (string, error) {
	if useDocker {
		if checkImageExists(isRemote, name) {
			return "", nil
		}
		return "", fmt.Errorf("image '%s' not found in Docker", name)
	}

	namespaces, err := listNamespaces(isRemote)
	if err != nil {
		return "", err
	}
	var targetNS []string
	for _, ns := range namespaces {
		if ns == preferNamespace {
			targetNS = append([]string{ns}, targetNS...)
		} else {
			targetNS = append(targetNS, ns)
		}
	}

	for _, ns := range targetNS {
		cmd, args := buildToolCmd(isRemote, ns, "image", "inspect", name)
		if exec.Command(cmd, args...).Run() == nil {
			return ns, nil
		}
	}
	return "", fmt.Errorf("image '%s' not found in any namespace", name)
}

// --- Infrastructure ---

func transferunregImage() error {
	if !checkImageExists(false, unregImage) {
		info("Unregistry image not found locally, pulling...")
		cmd, args := buildToolCmd(false, "", "pull", unregImage)
		if err := runCmdWithLiveOutput(cmd, args...); err != nil {
			return fmt.Errorf("failed to pull unregistry image locally: %w", err)
		}
	}

	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("unreg-%d.tar", time.Now().Unix()))
	info("Exporting unregistry image to %s...", tempFile)
	cmd, args := buildToolCmd(false, "", "save", "-o", tempFile, unregImage)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("failed to export unregistry image: %w", err)
	}
	defer os.Remove(tempFile)

	remoteFile := fmt.Sprintf("/tmp/unreg-%d.tar", time.Now().Unix())
	info("Transferring unregistry image to remote via SCP...")
	scpArgs := []string{tempFile, remoteHost + ":" + remoteFile}
	if *sshKey != "" {
		scpArgs = append([]string{"-i", *sshKey}, scpArgs...)
	}
	if err := runCmdWithLiveOutput("scp", scpArgs...); err != nil {
		return fmt.Errorf("failed to transfer image via SCP: %w", err)
	}

	info("Loading unregistry image on remote...")
	cmd, args = buildToolCmd(true, "", "load", "-i", remoteFile)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("failed to load unregistry image on remote: %w", err)
	}
	if err := runCmdWithLiveOutput("ssh", remoteHost, "rm -f "+remoteFile); err != nil {
		return fmt.Errorf("failed to remove remote file %s: %w", remoteFile, err)
	}
	return nil
}

func forwardPort(remotePort int) (int, error) {
	localPort := 55000 + int(time.Now().UnixNano()%10000)
	info("Establishing SSH tunnel: local:%d -> remote:%d", localPort, remotePort)
	sshArgs := make([]string, len(cachedSSHArgs))
	copy(sshArgs, cachedSSHArgs)
	sshArgs = append(sshArgs, "-L", fmt.Sprintf("%d:127.0.0.1:%d", localPort, remotePort), "-N")
	cmd := exec.Command("ssh", sshArgs...)
	sshTunnel = cmd
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	// Try to connect to the local port to verify tunnel is up
	for i := 0; i < 40; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return localPort, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return 0, fmt.Errorf("SSH tunnel timeout")
}

func startUnregistry(ns string) (int, error) {
	port := 55000 + int(time.Now().UnixNano()%10000)
	unregContainer = fmt.Sprintf("unreg-pussh-%d", time.Now().Unix())

	if !checkImageExists(true, unregImage) {
		if os.Getenv("UNREGISTRY_AIR_GAPPED") != "" {
			if err := transferunregImage(); err != nil {
				return 0, err
			}
		} else {
			info("Pulling unregistry image on remote...")
			cmd, args := buildToolCmd(true, "", "pull", unregImage)
			if err := runCmdWithLiveOutput(cmd, args...); err != nil {
				return 0, fmt.Errorf("failed to pull unregistry image on remote: %w", err)
			}
		}
	}

	info("Starting unregistry container '%s' on remote port %d...", unregContainer, port)
	cmd, args := buildToolCmd(true, "", "run", "-d", "--name", unregContainer, "-v", fmt.Sprintf("%s:/run/containerd/containerd.sock", remoteSocket), "--net", "host", "--userns=host", "--user", "root:root", unregImage, "--addr", fmt.Sprintf("127.0.0.1:%d", port))
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return 0, fmt.Errorf("failed to start unregistry container: %w", err)
	}
	return port, nil
}

func cleanup() {
	info("Cleaning up tunnel and remote container...")
	if sshTunnel != nil && sshTunnel.Process != nil {
		_ = sshTunnel.Process.Kill()
	}
	if unregContainer != "" {
		cmd, args := buildToolCmd(true, "", "rm", "-f", unregContainer)
		if err := runCmdWithLiveOutput(cmd, args...); err != nil {
			logPrintf("WARN", "Failed to remove unregistry container '%s': %v", unregContainer, err)
		}
	}
}

// --- Handlers ---

func handlePush() error {
	foundNs, err := findImage(false, imgName, imgNamespace)
	if err != nil {
		return err
	}

	rPort, err := startUnregistry(foundNs)
	if err != nil {
		return err
	}

	lPort, err := forwardPort(rPort)
	if err != nil {
		return err
	}

	localTag := fmt.Sprintf("localhost:%d/%s", lPort, imgName)
	remoteTag := fmt.Sprintf("localhost:%d/%s", rPort, imgName)

	info("Local - pushing image '%s' to registry via tunnel...", imgName)
	cmd, args := buildToolCmd(false, foundNs, "tag", imgName, localTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local tag failed: %w", err)
	}

	pushArgs := []string{"push"}
	if useDocker && *platform != "" {
		pushArgs = append(pushArgs, "--platform", *platform)
	}
	pushArgs = append(pushArgs, localTag)
	cmd, args = buildToolCmd(false, foundNs, pushArgs...)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local push failed: %w", err)
	}

	cmd, args = buildToolCmd(false, foundNs, "rmi", localTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local rmi failed: %w", err)
	}

	// The tag/rmi workflow can be bypassed to save approximately 1s if the containerd.snapshotter mechanism is enabled.
	// Note that unlike "docker info", "nerdctl info" provides no reliable way to detect this state.
	info("Remote - pulling image '%s' from registry...", imgName)
	cmd, args = buildToolCmd(true, imgNamespace, "pull", remoteTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote pull failed: %w", err)
	}

	cmd, args = buildToolCmd(true, imgNamespace, "tag", remoteTag, imgName)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote tag failed: %w", err)
	}

	cmd, args = buildToolCmd(true, imgNamespace, "rmi", remoteTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote rmi failed: %w", err)
	}
	info("Push successful.")
	return nil
}

func handlePull() error {
	foundNs, err := findImage(true, imgName, imgNamespace)
	if err != nil {
		return err
	}

	rPort, err := startUnregistry(foundNs)
	if err != nil {
		return err
	}

	lPort, err := forwardPort(rPort)
	if err != nil {
		return err
	}

	localTag := fmt.Sprintf("localhost:%d/%s", lPort, imgName)
	remoteTag := fmt.Sprintf("localhost:%d/%s", rPort, imgName)

	info("Remote - pushing image '%s' to registry...", imgName)
	cmd, args := buildToolCmd(true, foundNs, "tag", imgName, remoteTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote tag failed: %w", err)
	}

	cmd, args = buildToolCmd(true, foundNs, "push", remoteTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote push failed: %w", err)
	}

	cmd, args = buildToolCmd(true, foundNs, "rmi", remoteTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("remote rmi failed: %w", err)
	}

	info("Local - pulling image '%s' from registry via tunnel...", imgName)
	cmd, args = buildToolCmd(false, imgNamespace, "pull", localTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local pull failed: %w", err)
	}

	cmd, args = buildToolCmd(false, imgNamespace, "tag", localTag, imgName)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local tag failed: %w", err)
	}

	cmd, args = buildToolCmd(false, imgNamespace, "rmi", localTag)
	if err := runCmdWithLiveOutput(cmd, args...); err != nil {
		return fmt.Errorf("local rmi failed: %w", err)
	}
	info("Pull successful.")
	return nil
}

// --- Detection ---

func checkTool(isRemote bool) (string, string, error) {
	toolName := "nerdctl"
	if useDocker {
		toolName = "docker"
	}

	var paths []string
	whichCmd := "which " + toolName
	if out, err := (func() ([]byte, error) {
		if isRemote {
			return exec.Command("ssh", remoteHost, whichCmd).Output()
		}
		return exec.Command("sh", "-c", whichCmd).Output()
	})(); err == nil {
		paths = append(paths, strings.TrimSpace(string(out)))
	}

	for _, dir := range toolDirectories {
		paths = append(paths, filepath.Join(dir, toolName))
	}

	for _, p := range paths {
		testCmd := p + " image ls"
		if !useDocker {
			if isRemote {
				testCmd += fmt.Sprintf(" --address %s", remoteSocket)
			} else {
				testCmd += fmt.Sprintf(" --address %s", localSocket)
			}
		}
		debug("Testing tool path %s", p)
		if isRemote {
			sshArgs := make([]string, len(cachedSSHArgs))
			copy(sshArgs, cachedSSHArgs)
			sshArgs = append(sshArgs, testCmd)
			if exec.Command("ssh", sshArgs...).Run() == nil {
				return p, "", nil
			}
			sshArgsSudo := make([]string, len(cachedSSHArgs))
			copy(sshArgsSudo, cachedSSHArgs)
			sshArgsSudo = append(sshArgsSudo, "sudo -n "+testCmd)
			if exec.Command("ssh", sshArgsSudo...).Run() == nil {
				return p, "sudo -n", nil
			}
		} else {
			if exec.Command("sh", "-c", testCmd).Run() == nil {
				return p, "", nil
			}
			if exec.Command("sh", "-c", "sudo -n "+testCmd).Run() == nil {
				return p, "sudo -n", nil
			}
		}
	}
	return "", "", fmt.Errorf("tool '%s' not found or accessible", toolName)
}

func run() error {
	image, host := flag.Arg(0), flag.Arg(1)
	useDocker = !strings.Contains(image, "::")
	imgName = image
	if !useDocker {
		parts := strings.SplitN(image, "::", 2)
		imgNamespace, imgName = parts[0], parts[1]
	}
	remoteHost = host
	cachedSSHArgs = buildSSHArgs()

	// Use defer to ensure cleanup happens even if an error occurs later
	defer cleanup()

	info("Detecting local environment...")
	localSocket, _ = findContainerdSocket(false)
	var err error
	localTool, localToolSudo, err = checkTool(false)
	if err != nil {
		return err
	}
	info("Local tool: %s (sudo: %v), socket: %s", localTool, localToolSudo != "", localSocket)

	info("Detecting remote environment on %s...", host)
	remoteSocket, _ = findContainerdSocket(true)
	remoteTool, remoteToolSudo, err = checkTool(true)
	if err != nil {
		return err
	}
	info("Remote tool: %s (sudo: %v), socket: %s", remoteTool, remoteToolSudo != "", remoteSocket)

	if *pull {
		return handlePull()
	}
	return handlePush()
}

func main() {
	flag.Usage = func() {
		fmt.Print(`pussh - Push/pull container images via SSH without external registries.

USAGE: pussh [OPTIONS] IMAGE HOST

OPTIONS:
  -h, --help                Show help
  --version                 Show version
  --verbose                 Enable debug logging
  --pull                    Pull mode (remote to local)
  -i, --ssh-key             Path to SSH private key
  --no-host-key-check       Skip SSH host key checking
  --platform                Push a specific platform for multi-platform images
`)
	}
	flag.Parse()

	// Ensure localhost connections bypass proxy.
	os.Setenv("no_proxy", os.Getenv("no_proxy")+",localhost,127.0.0.1")

	if *version {
		fmt.Printf("pussh v%s\n", VERSION)
		return
	}
	if *help || flag.NArg() < 2 {
		flag.Usage()
		return
	}

	// Execute core logic and capture errors to ensure defers run
	if err := run(); err != nil {
		logPrintf("FATA", "%v", err)
	}
}
