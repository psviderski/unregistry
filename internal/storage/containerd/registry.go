package containerd

import (
	"context"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
)

// registry implements distribution.Namespace backed by containerd image store.
type registry struct {
	// TODO: change to regular containerd Client.
	client *Client
}

// Ensure registry implements distribution.registry.
var _ distribution.Namespace = &registry{}

// Scope returns the global scope for this registry.
func (n *registry) Scope() distribution.Scope {
	return distribution.GlobalScope
}

// Repository returns an instance of repository for the given name.
func (n *registry) Repository(_ context.Context, name reference.Named) (distribution.Repository, error) {
	return newRepository(n.client, name), nil
}

// Repositories returns a list of repositories.
func (n *registry) Repositories(ctx context.Context, repos []string, last string) (int, error) {
	// For now, we don't support listing repositories.
	// This would require iterating through all images in containerd and extracting unique repository names.
	return 0, distribution.ErrUnsupported
}

// Blobs returns a blob enumerator.
func (n *registry) Blobs() distribution.BlobEnumerator {
	return &blobEnumerator{
		client: n.client,
	}
}

// BlobStatter returns a blob statter.
func (n *registry) BlobStatter() distribution.BlobStatter {
	return &blobStatter{
		client: n.client,
	}
}

// blobEnumerator implements distribution.BlobEnumerator.
type blobEnumerator struct {
	client *Client
}

// Enumerate is not supported for containerd backend.
func (e *blobEnumerator) Enumerate(ctx context.Context, ingester func(digest.Digest) error) error {
	// We don't support blob enumeration for now.
	return distribution.ErrUnsupported
}

// blobStatter implements distribution.BlobStatter.
type blobStatter struct {
	client *Client
}

// Stat returns the descriptor for a blob.
func (s *blobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	ctx = s.client.Context(ctx)
	info, err := s.client.ContentStore().Info(ctx, dgst)
	if err != nil {
		return distribution.Descriptor{}, convertError(err)
	}

	return distribution.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    info.Digest,
		Size:      info.Size,
	}, nil
}
