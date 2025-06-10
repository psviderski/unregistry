package containerd

import (
	"context"

	"github.com/containerd/containerd/v2/client"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
)

// repository implements distribution.Repository backed by the containerd content and image stores.
type repository struct {
	client    *client.Client
	name      reference.Named
	blobStore *blobStore
}

var _ distribution.Repository = &repository{}

func newRepository(client *client.Client, name reference.Named) *repository {
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

// Manifests returns the manifest service for the repository backed by the containerd content store.
func (r *repository) Manifests(
	_ context.Context, _ ...distribution.ManifestServiceOption,
) (distribution.ManifestService, error) {
	return &manifestService{
		repo:      r.name,
		blobStore: r.blobStore,
	}, nil
}

// Blobs returns the blob store for the repository backed by the containerd content store.
func (r *repository) Blobs(_ context.Context) distribution.BlobStore {
	return r.blobStore
}

// Tags returns the tag service for the repository backed by the containerd image store.
func (r *repository) Tags(_ context.Context) distribution.TagService {
	// Shouldn't return an error as r.name is a valid reference.
	canonicalRepo, _ := reference.ParseNormalizedNamed(r.name.String())
	return &tagService{
		client:        r.client,
		canonicalRepo: canonicalRepo,
	}
}
