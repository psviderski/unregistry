package containerd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

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

// Open returns a reader for the blob.
// TODO
func (b *blobStore) Open(ctx context.Context, dgst digest.Digest) (io.ReadSeekCloser, error) {
	reader, err := b.client.ContentStore().ReaderAt(ctx, ocispec.Descriptor{Digest: dgst})
	if err != nil {
		// TODO: convert err if possible
		return nil, err
	}

	// Wrap the ReaderAt as a ReadSeekCloser
	return &readerAtWrapper{
		readerAt: reader,
		size:     reader.Size(),
		offset:   0,
	}, nil
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
func (b *blobStore) Mount(ctx context.Context, sourceRepo reference.Named, dgst digest.Digest) (
	distribution.Descriptor, error,
) {
	// We could implement cross-repository mounting here by checking if the blob exists and returning its descriptor.
	// The content in containerd is not repository-namespaced. But for now we'll keep it simple.
	return distribution.Descriptor{}, distribution.ErrUnsupported
}

// ServeBlob serves the blob over HTTP.
// TODO
func (b *blobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	reader, err := b.Open(ctx, dgst)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Get blob info for size
	desc, err := b.Stat(ctx, dgst)
	if err != nil {
		return err
	}

	// Set headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", dgst.String())
	w.Header().Set("Content-Length", fmt.Sprintf("%d", desc.Size))

	// Serve the content with range support
	http.ServeContent(w, r, "", time.Time{}, reader)
	return nil
}

// Delete removes a blob.
func (b *blobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	// For now, we don't support deletion to keep things simple
	// Containerd's garbage collection should handle cleanup
	return distribution.ErrUnsupported
}

// readerAtWrapper wraps a content.ReaderAt to implement io.ReadSeekCloser.
type readerAtWrapper struct {
	readerAt content.ReaderAt
	size     int64
	offset   int64
}

func (r *readerAtWrapper) Read(p []byte) (int, error) {
	n, err := r.readerAt.ReadAt(p, r.offset)
	r.offset += int64(n)
	if err == io.EOF && r.offset == r.size {
		return n, io.EOF
	}
	return n, err
}

func (r *readerAtWrapper) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset += offset
	case io.SeekEnd:
		r.offset = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if r.offset < 0 {
		r.offset = 0
		return 0, fmt.Errorf("negative position")
	}
	if r.offset > r.size {
		r.offset = r.size
	}

	return r.offset, nil
}

func (r *readerAtWrapper) Close() error {
	return r.readerAt.Close()
}
