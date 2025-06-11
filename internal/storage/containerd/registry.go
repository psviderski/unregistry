package containerd

import (
	"context"

	"github.com/containerd/containerd/v2/client"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
)

// registry implements distribution.Namespace backed by containerd image store.
type registry struct {
	client *client.Client
}

// Ensure registry implements distribution.registry.
var _ distribution.Namespace = &registry{}

// Scope returns the global scope for this registry.
func (r *registry) Scope() distribution.Scope {
	return distribution.GlobalScope
}

// Repository returns an instance of repository for the given name.
func (r *registry) Repository(_ context.Context, name reference.Named) (distribution.Repository, error) {
	return newRepository(r.client, name), nil
}

// Repositories should return a list of repositories in the registry but it's not supported for simplicity.
func (r *registry) Repositories(_ context.Context, _ []string, _ string) (int, error) {
	return 0, distribution.ErrUnsupported
}

// Blobs returns a stub implementation of distribution.BlobEnumerator that doesn't support enumeration.
func (r *registry) Blobs() distribution.BlobEnumerator {
	return &unsupportedBlobEnumerator{}
}

// BlobStatter returns a blob store that can stat blobs in the containerd content store.
// It doesn't seem BlobStatter is used in distribution, but it's part of the interface.
func (r *registry) BlobStatter() distribution.BlobStatter {
	return &blobStore{
		client: r.client,
	}
}

// unsupportedBlobEnumerator implements distribution.BlobEnumerator but doesn't support enumeration.
type unsupportedBlobEnumerator struct{}

// Enumerate is not supported for containerd backend for now.
// It looks like distribution.BlobEnumerator is used for garbage collection, but we don't need that because containerd
// has its own garbage collection mechanism that works with content store directly.
func (e *unsupportedBlobEnumerator) Enumerate(_ context.Context, _ func(digest.Digest) error) error {
	return distribution.ErrUnsupported
}
