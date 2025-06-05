package containerd

import (
	"context"
	"errors"
	"fmt"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/errdefs"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/distribution/distribution/v3/manifest/ocischema"
	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

// manifestService implements distribution.ManifestService backed by containerd.
type manifestService struct {
	client    *Client
	repo      reference.Named
	blobStore distribution.BlobStore
}

// Exists checks if a manifest exists in containerd content store by digest.
func (m *manifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	return content.Exists(ctx, m.client.ContentStore(), ocispec.Descriptor{Digest: dgst})
}

// Get retrieves a manifest by digest.
func (m *manifestService) Get(
	ctx context.Context, dgst digest.Digest, _ ...distribution.ManifestServiceOption,
) (distribution.Manifest, error) {
	blob, err := m.blobStore.Get(ctx, dgst)
	if err != nil {
		if errors.Is(err, distribution.ErrBlobUnknown) {
			return nil, distribution.ErrManifestUnknownRevision{
				Name:     m.repo.Name(),
				Revision: dgst,
			}
		}
		return nil, err
	}

	manifest, err := unmarshalManifest(blob)
	if err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	if mediaType, _, err := manifest.Payload(); err == nil {
		logrus.WithFields(
			logrus.Fields{
				"repo":      m.repo.Name(),
				"digest":    dgst,
				"mediatype": mediaType,
			},
		).Debug("Got manifest from containerd content store.")
	}

	return manifest, nil
}

// Put stores a manifest.
func (m *manifestService) Put(
	ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption,
) (digest.Digest, error) {
	ctx = m.client.Context(ctx)

	// Marshal the manifest
	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return "", fmt.Errorf("failed to get manifest payload: %w", err)
	}

	// Calculate digest
	dgst := digest.FromBytes(payload)

	// Create a lease to prevent garbage collection during the operation
	lease, err := m.client.LeasesService().Create(ctx, leases.WithRandomID())
	if err != nil {
		return "", fmt.Errorf("failed to create lease: %w", err)
	}
	defer m.client.LeasesService().Delete(ctx, lease)

	// Write the manifest to the content store
	ref := fmt.Sprintf("%s@%s", m.repo.String(), dgst)
	writer, err := m.client.ContentStore().Writer(
		ctx,
		content.WithRef(ref),
		content.WithDescriptor(
			ocispec.Descriptor{
				MediaType: mediaType,
				Digest:    dgst,
				Size:      int64(len(payload)),
			},
		),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create content writer: %w", err)
	}

	if _, err := writer.Write(payload); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := writer.Commit(ctx, 0, dgst); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return "", fmt.Errorf("failed to commit manifest: %w", err)
		}
	}

	// If this is a tag operation (from docker push), update the image store
	for _, option := range options {
		if opt, ok := option.(distribution.WithTagOption); ok {
			tag := opt.Tag
			if err := m.updateImageStore(ctx, m.repo, tag, dgst, mediaType); err != nil {
				return "", fmt.Errorf("failed to update image store: %w", err)
			}
		}
	}

	return dgst, nil
}

// Delete removes a manifest by digest.
func (m *manifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	ctx = m.client.Context(ctx)

	// For now, we don't support deletion to keep things simple
	// Containerd's garbage collection should handle cleanup
	return distribution.ErrUnsupported
}

// updateImageStore updates the containerd image store with the manifest.
func (m *manifestService) updateImageStore(
	ctx context.Context, repo reference.Named, tag string, dgst digest.Digest, mediaType string,
) error {
	// Create the image reference
	ref := fmt.Sprintf("%s:%s", repo.String(), tag)

	// Create the image
	img := images.Image{
		Name: ref,
		Target: ocispec.Descriptor{
			MediaType: mediaType,
			Digest:    dgst,
			Size:      0, // Will be filled by containerd
		},
	}

	// Update or create the image
	_, err := m.client.ImageStore().Update(ctx, img)
	if err != nil {
		// If update fails, try to create
		_, err = m.client.ImageStore().Create(ctx, img)
		if err != nil && !errdefs.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}

// unmarshalManifest attempts to unmarshal a manifest in various formats.
func unmarshalManifest(blob []byte) (distribution.Manifest, error) {
	// Try OCI manifest.
	var ociManifest ocischema.DeserializedManifest
	if err := ociManifest.UnmarshalJSON(blob); err == nil {
		return &ociManifest, nil
	}

	// Try Docker schema2 manifest.
	var schema2Manifest schema2.DeserializedManifest
	if err := schema2Manifest.UnmarshalJSON(blob); err == nil {
		return &schema2Manifest, nil
	}

	// Try manifest list (OCI index or Docker manifest list).
	var manifestList manifestlist.DeserializedManifestList
	if err := manifestList.UnmarshalJSON(blob); err == nil {
		return &manifestList, nil
	}

	return nil, fmt.Errorf("unknown manifest format")
}
