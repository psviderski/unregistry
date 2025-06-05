package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/namespaces"
)

// Client wraps a containerd client with registry-specific functionality.
type Client struct {
	client    *client.Client
	namespace string
}

// NewClient creates a new containerd client.
func NewClient(address, namespace string) (*Client, error) {
	if address == "" {
		address = "/run/containerd/containerd.sock"
	}
	if namespace == "" {
		namespace = "moby"
	}

	c, err := client.New(address, client.WithDefaultNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &Client{
		client:    c,
		namespace: namespace,
	}, nil
}

// Close closes the containerd client connection.
func (c *Client) Close() error {
	return c.client.Close()
}

// ImageStore returns the image store for the namespace.
func (c *Client) ImageStore() images.Store {
	return c.client.ImageService()
}

// ContentStore returns the content store for the namespace.
func (c *Client) ContentStore() content.Store {
	return c.client.ContentStore()
}

// LeasesService returns the leases service for the namespace.
func (c *Client) LeasesService() leases.Manager {
	return c.client.LeasesService()
}

// Context returns a context with the namespace set.
func (c *Client) Context(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, c.namespace)
}