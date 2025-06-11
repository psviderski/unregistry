package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/uncloud/unregistry/internal/registry"
)

func main() {
	var cfg registry.Config
	cmd := &cobra.Command{
		Use:   "unregistry",
		Short: "A container registry that uses local Docker/containerd for storing images.",
		Long: `Unregistry is a lightweight OCI-compliant container registry that uses the local Docker (containerd)
image store as its backend. It provides a standard registry API interface for pushing and pulling
container images without requiring a separate storage backend.

Key use cases:
- Push built images straight to remote servers without an external registry such as Docker Hub
  as intermediary
- Pull images once and serve them to multiple nodes in a cluster environment
- Distribute images in air-gapped environments
- Development and testing workflows that need a local registry
- Expose pre-loaded images through a standard registry API`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			bindEnvToFlag(cmd, "addr", "UNREGISTRY_ADDR")
			bindEnvToFlag(cmd, "log-format", "UNREGISTRY_LOG_FORMAT")
			bindEnvToFlag(cmd, "log-level", "UNREGISTRY_LOG_LEVEL")
			bindEnvToFlag(cmd, "namespace", "UNREGISTRY_CONTAINERD_NAMESPACE")
			bindEnvToFlag(cmd, "socket", "UNREGISTRY_CONTAINERD_SOCK")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	cmd.Flags().StringVarP(&cfg.Addr, "addr", "a", ":5000",
		"Address and port to listen on (e.g., 0.0.0.0:5000)")
	cmd.Flags().StringVarP(&cfg.LogFormatter, "log-format", "f", "text",
		"Log output format (text or json)")
	cmd.Flags().StringVarP(&cfg.LogLevel, "log-level", "l", "info",
		"Log verbosity level (debug, info, warn, error)")
	cmd.Flags().StringVarP(&cfg.ContainerdNamespace, "namespace", "n", "moby",
		"Containerd namespace to use for image storage")
	cmd.Flags().StringVarP(&cfg.ContainerdSock, "sock", "s", "/run/containerd/containerd.sock",
		"Path to containerd socket file")

	if err := cmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("Registry server failed.")
	}
}

func run(cfg registry.Config) error {
	reg, err := registry.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("create registry server: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := reg.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err = <-errCh:
		return err
	case <-quit:
		timeout := 30 * time.Second
		logrus.Infof("Shutting down server... Draining connections for %s", timeout)
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err = reg.Shutdown(ctx); err != nil {
			return fmt.Errorf("registry server forced to shutdown: %w", err)
		}
		logrus.Info("Registry server stopped gracefully.")
	}

	return nil
}

func bindEnvToFlag(cmd *cobra.Command, flagName, envVar string) {
	if value := os.Getenv(envVar); value != "" && !cmd.Flags().Changed(flagName) {
		if err := cmd.Flags().Set(flagName, value); err != nil {
			logrus.WithError(err).Fatalf("Failed to bind environment variable '%s' to flag '%s'.", envVar, flagName)
		}
	}
}
