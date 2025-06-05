package containerd

import (
	"context"
	"fmt"

	"github.com/distribution/distribution/v3"
	middleware "github.com/distribution/distribution/v3/registry/middleware/registry"
	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
)

const MiddlewareName = "containerd"

func init() {
	// Register the containerd middleware. In fact, this is not a middleware but a self-sufficient registry
	// implementation that uses containerd as the backend for storing images. It seems that using middleware
	// is the only way to register a custom registry in the distribution package.
	err := middleware.Register(MiddlewareName, registryMiddleware)
	if err != nil {
		panic(fmt.Sprintf("failed to register containerd middleware: %v", err))
	}
}

// registryMiddleware is the registry middleware factory function that creates an instance of registry.
func registryMiddleware(
	_ context.Context, _ distribution.Namespace, _ storagedriver.StorageDriver, options map[string]interface{},
) (distribution.Namespace, error) {
	sock, ok := options["sock"].(string)
	if !ok || sock == "" {
		return nil, fmt.Errorf("containerd socket path is required")
	}
	namespace, ok := options["namespace"].(string)
	if !ok || namespace == "" {
		return nil, fmt.Errorf("containerd namespace is required")
	}

	// TODO: create regular containerd Client instead of using the custom one.
	// Create containerd client
	client, err := NewClient(sock, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd client: %w", err)
	}

	return &registry{client: client}, nil
}
