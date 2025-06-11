package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry/handlers"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/filesystem"
	"github.com/sirupsen/logrus"
	"github.com/uncloud/unregistry/internal/storage/containerd"
	_ "github.com/uncloud/unregistry/internal/storage/containerd"
)

// Registry represents a complete instance of the registry.
type Registry struct {
	app    *handlers.App
	server *http.Server
}

// NewRegistry creates a new registry from the given configuration.
func NewRegistry(cfg Config) (*Registry, error) {
	// Configure logging.
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	logrus.SetLevel(level)

	switch cfg.LogFormatter {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{})
	default:
		return nil, fmt.Errorf("invalid log formatter: '%s'; expected 'json' or 'text'", cfg.LogFormatter)
	}

	distConfig := &configuration.Configuration{
		Storage: configuration.Storage{
			"filesystem": configuration.Parameters{
				"rootdirectory": "/tmp/registry", // Dummy storage driver
			},
		},
		Middleware: map[string][]configuration.Middleware{
			"registry": {
				{
					Name: containerd.MiddlewareName,
					Options: configuration.Parameters{
						"namespace": cfg.ContainerdNamespace,
						"sock":      cfg.ContainerdSock,
					},
				},
			},
		},
	}
	app := handlers.NewApp(context.Background(), distConfig)
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: app,
	}

	return &Registry{
		app:    app,
		server: server,
	}, nil
}

// ListenAndServe starts the HTTP server for the registry.
func (r *Registry) ListenAndServe() error {
	logrus.WithField("addr", r.server.Addr).Info("Starting registry server.")
	if err := r.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the registry's HTTP server and application object.
func (r *Registry) Shutdown(ctx context.Context) error {
	err := r.server.Shutdown(ctx)
	if appErr := r.app.Shutdown(); appErr != nil {
		err = errors.Join(err, appErr)
	}
	return err
}
