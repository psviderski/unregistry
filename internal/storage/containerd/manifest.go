package containerd

import (
	"context"
	"errors"
	"fmt"

	"github.com/distribution/distribution/v3"
	"github.com/distribution/distribution/v3/manifest/manifestlist"
	"github.com/distribution/distribution/v3/manifest/ocischema"
	"github.com/distribution/distribution/v3/manifest/schema2"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// manifestService implements distribution.ManifestService backed by containerd content store.
type manifestService struct {
	repo      reference.Named
	blobStore *blobStore
}

// Exists checks if a manifest exists in the blob store by digest.
func (m *manifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	_, err := m.blobStore.Stat(ctx, dgst)
	if errors.Is(err, distribution.ErrBlobUnknown) {
		return false, nil
	}
	return err == nil, err
}

// Get retrieves a manifest from the blob store by its digest.
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
		).Debug("Got manifest from blob store.")
	}

	return manifest, nil
}

// Put stores a manifest in the blob store and returns its digest.
func (m *manifestService) Put(
	ctx context.Context, manifest distribution.Manifest, _ ...distribution.ManifestServiceOption,
) (digest.Digest, error) {
	mediaType, payload, err := manifest.Payload()
	if err != nil {
		return "", fmt.Errorf("get manifest payload: %w", err)
	}

	desc, err := m.blobStore.Put(ctx, mediaType, payload)
	if err != nil {
		return "", fmt.Errorf("put manifest in blob store: %w", err)
	}

	return desc.Digest, nil
}

// Delete is not supported to keep things simple.
func (m *manifestService) Delete(_ context.Context, _ digest.Digest) error {
	return distribution.ErrUnsupported
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

	return nil, distribution.ErrManifestVerification{errors.New("unknown manifest format")}
}
