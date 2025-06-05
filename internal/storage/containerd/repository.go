package containerd

import (
	"context"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
)

// repository implements distribution.Repository backed by containerd image store.
type repository struct {
	client    *Client
	name      reference.Named
	blobStore distribution.BlobStore
}

func newRepository(client *Client, name reference.Named) *repository {
	return &repository{
		client: client,
		name:   name,
		blobStore: &blobStore{
			client: client,
			repo:   name,
		},
	}
}

// Named returns the name of the repository.
func (r *repository) Named() reference.Named {
	return r.name
}

// Manifests returns the manifest service for the repository.
func (r *repository) Manifests(
	_ context.Context, _ ...distribution.ManifestServiceOption,
) (distribution.ManifestService, error) {
	return &manifestService{
		client:    r.client,
		repo:      r.name,
		blobStore: r.blobStore,
	}, nil
}

// Blobs returns the blob store for the repository.
func (r *repository) Blobs(_ context.Context) distribution.BlobStore {
	return r.blobStore
}

// Tags returns the tag service for the repository.
func (r *repository) Tags(_ context.Context) distribution.TagService {
	return &tagService{
		client: r.client,
		repo:   r.name,
	}
}
