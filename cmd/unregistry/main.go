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
		Use:           "unregistry",
		Short:         "A container registry backed by the local Docker/containerd image store.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			bindEnvToFlag(cmd, "addr", "UNREGISTRY_ADDR")
			bindEnvToFlag(cmd, "log-level", "UNREGISTRY_LOG_LEVEL")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	cmd.Flags().StringVar(&cfg.HTTPAddr, "addr", ":5000", "HTTP server address")
	cmd.Flags().StringVar(&cfg.LogLevel, "log-level", "info", "Log level (debug, info, warn, error).")
	//cmd.Flags().String("secret", "", "HTTP secret key")

	if err := cmd.Execute(); err != nil {
		logrus.WithError(err).Fatal("Registry server failed.")
	}
}

func run(cfg registry.Config) error {
	reg, err := registry.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("create registry server: %w", err)
	}

	errCh := make(chan error)
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
