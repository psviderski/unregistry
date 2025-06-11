package containerd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// blobStore implements distribution.BlobStore backed by containerd image store.
type blobStore struct {
	client *client.Client
	repo   reference.Named
}

// Stat returns metadata about a blob in the containerd content store by its digest.
// If the blob doesn't exist, distribution.ErrBlobUnknown will be returned.
func (b *blobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	info, err := b.client.ContentStore().Info(ctx, dgst)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		}
		return distribution.Descriptor{}, fmt.Errorf(
			"get metadata for blob '%s' from containerd content store: %w", dgst, err,
		)
	}

	return distribution.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    info.Digest,
		Size:      info.Size,
	}, nil
}

// Get retrieves the content of a blob in the containerd content store by its digest.
// If the blob doesn't exist, distribution.ErrBlobUnknown will be returned.
func (b *blobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	blob, err := content.ReadBlob(ctx, b.client.ContentStore(), ocispec.Descriptor{Digest: dgst})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, distribution.ErrBlobUnknown
		}
		return nil, fmt.Errorf("read blob '%s' from containerd content store: %w", dgst, err)
	}

	return blob, nil
}

// Open returns a reader for the blob in the containerd content store by its digest.
func (b *blobStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
	reader, err := newBlobReadSeekCloser(ctx, b.client.ContentStore(), ocispec.Descriptor{Digest: dgst})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, distribution.ErrBlobUnknown
		}
		return nil, fmt.Errorf("open blob '%s' from containerd content store: %w", dgst, err)
	}

	return reader, nil
}

// Put stores a blob in the containerd content store with the given media type. If the blob already exists,
// it will return the existing descriptor without re-uploading the content. It should be used for small objects,
// such as manifests.
func (b *blobStore) Put(ctx context.Context, mediaType string, blob []byte) (distribution.Descriptor, error) {
	writer, err := newBlobWriter(ctx, b.client, b.repo, "")
	if err != nil {
		return distribution.Descriptor{}, err
	}
	defer func() {
		if err != nil {
			// Clean up resources occupied by the writer if an error occurs.
			_ = writer.Cancel(ctx)
		}
		writer.Close()
	}()

	if _, err = writer.Write(blob); err != nil {
		return distribution.Descriptor{}, err
	}

	desc := distribution.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(blob),
		Size:      int64(len(blob)),
	}
	if desc, err = writer.Commit(ctx, desc); err != nil {
		return distribution.Descriptor{}, err
	}

	return desc, nil
}

// Create creates a blob writer to add a blob to the containerd content store.`
func (b *blobStore) Create(ctx context.Context, _ ...distribution.BlobCreateOption) (
	distribution.BlobWriter, error,
) {
	return newBlobWriter(ctx, b.client, b.repo, "")
}

// Resume creates a blob writer for resuming an upload with a specific ID.
func (b *blobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	return newBlobWriter(ctx, b.client, b.repo, id)
}

// Mount is not supported for simplicity.
// We could implement cross-repository mounting here by checking if the blob exists and returning its descriptor.
// However, the content in containerd is not repository-namespaced so checking if a blob exists in a new repository
// will return true if it exists in the content store, regardless of the repository. Given that, we don't really
// need the mount operation in this implementation.
func (b *blobStore) Mount(ctx context.Context, sourceRepo reference.Named, dgst digest.Digest) (
	distribution.Descriptor, error,
) {
	return distribution.Descriptor{}, distribution.ErrUnsupported
}

// ServeBlob serves the blob from containerd content store over HTTP.
func (b *blobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	// Get the blob info to check if it exists and populate the response headers.
	desc, err := b.Stat(ctx, dgst)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", desc.MediaType)
	w.Header().Set("Content-Length", strconv.FormatInt(desc.Size, 10))
	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.Header().Set("Etag", dgst.String())

	if r.Method == http.MethodHead {
		return nil
	}

	reader, err := b.Open(ctx, dgst)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.CopyN(w, reader, desc.Size)
	return err
}

// Delete is not supported for simplicity.
// Deletion can be done by deleting images in containerd, which will clean up the blobs.
func (b *blobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return distribution.ErrUnsupported
}

// blobReadSeekCloser is an io.ReadSeekCloser that wraps a content.ReaderAt.
type blobReadSeekCloser struct {
	*io.SectionReader
	ra content.ReaderAt
}

func newBlobReadSeekCloser(ctx context.Context, provider content.Provider, desc ocispec.Descriptor) (
	io.ReadSeekCloser, error,
) {
	ra, err := provider.ReaderAt(ctx, desc)
	if err != nil {
		return nil, err
	}

	return &blobReadSeekCloser{
		SectionReader: io.NewSectionReader(ra, 0, ra.Size()),
		ra:            ra,
	}, nil
}

func (rsc *blobReadSeekCloser) Close() error {
	return rsc.ra.Close()
}
