package containerd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/errdefs"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
)

const leaseExpiration = 1 * time.Hour

// blobWriter is a resumable blob uploader to the containerd content store.
// Implements distribution.BlobWriter.
type blobWriter struct {
	client *client.Client
	repo   reference.Named
	id     string

	// lease is a containerd lease for writer that prevents garbage collection of the content. It's intentionally not
	// deleted on successful blob commit to keep it while the registry is uploading other blobs and manifests and
	// creating an image referencing them. Otherwise, the blob would be garbage collected immediately after lease is
	// deleted if the blob is not referenced by an image.
	// In the worst case, the lease and unreferenced blob will be garbage collected after leaseExpiration.
	lease  leases.Lease
	writer content.Writer
	// size is the total number of bytes written to writer.
	size int64
	log  *logrus.Entry
}

func newBlobWriter(
	ctx context.Context, client *client.Client, repo reference.Named, id string,
) (distribution.BlobWriter, error) {
	if id == "" {
		id = uuid.NewString()
	}

	// Create a containerd lease to prevent garbage collection.
	opts := []leases.Opt{
		leases.WithRandomID(),
		leases.WithExpiration(leaseExpiration),
	}
	lease, err := client.LeasesService().Create(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create containerd lease: %w", err)
	}

	// Open a containerd content writer with the lease.
	ctx = leases.WithLease(ctx, lease.ID)
	writer, err := content.OpenWriter(ctx, client.ContentStore(), content.WithRef("upload-"+id))
	if err != nil {
		_ = client.LeasesService().Delete(ctx, lease)
		return nil, fmt.Errorf("create containerd content writer: %w", err)
	}

	// Get the status of the writer to get the written offset (size) if the writer was resumed.
	status, err := writer.Status()
	if err != nil {
		return nil, fmt.Errorf("get containerd content writer status: %w", err)
	}

	log := logrus.WithFields(
		logrus.Fields{
			"writer.id": id,
			"repo":      repo.Name(),
		},
	)
	log.WithField("size", status.Offset).Debug("Created new containerd blob writer.")

	return &blobWriter{
		client: client,
		repo:   repo,
		id:     id,
		lease:  lease,
		writer: writer,
		size:   status.Offset,
		log:    log,
	}, nil
}

// ID returns the identifier for this blob upload.
func (bw *blobWriter) ID() string {
	return bw.id
}

// StartedAt returns the time the upload started.
func (bw *blobWriter) StartedAt() time.Time {
	return bw.lease.CreatedAt
}

// Size returns the number of bytes written to the containerd blob writer.
func (bw *blobWriter) Size() int64 {
	return bw.size
}

// ReadFrom reads from the provided reader and writes to the containerd blob writer.
func (bw *blobWriter) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.Copy(bw.writer, r)
	bw.size += n

	log := bw.log.WithField("size", n)
	if err != nil {
		err = fmt.Errorf("copy data to containerd blob writer: %w", err)
		log = log.WithError(err)
	}
	log.Debug("Copied data to containerd blob writer.")

	return n, err
}

// Write writes data to the containerd blob writer.
func (bw *blobWriter) Write(data []byte) (int, error) {
	n, err := bw.writer.Write(data)
	bw.size += int64(n)

	log := bw.log.WithField("size", n)
	if err != nil {
		err = fmt.Errorf("write data to containerd blob writer: %w", err)
		log = log.WithError(err)
	}
	log.Debug("Wrote data to containerd blob writer.")

	return n, err
}

// Commit finalizes the blob upload.
func (bw *blobWriter) Commit(ctx context.Context, desc distribution.Descriptor) (distribution.Descriptor, error) {
	log := bw.log.WithFields(
		logrus.Fields{
			"digest":    desc.Digest,
			"mediatype": desc.MediaType,
			"size":      bw.size,
		},
	)

	log.Debug("Committing blob to containerd content store.")
	// The caller may not provide a size in the descriptor if it doesn't know it so we use the calculated size from
	// the writer.
	if err := bw.writer.Commit(ctx, bw.size, desc.Digest); err != nil {
		// The writer didn't create a new blob so we don't need to keep the lease.
		_ = bw.client.LeasesService().Delete(ctx, bw.lease)

		if errdefs.IsAlreadyExists(err) {
			log.Debug("Blob already exists in containerd content store.")
		} else {
			return distribution.Descriptor{}, fmt.Errorf("commit blob to containerd content store: %w", err)
		}
	} else {
		log.Debug("Successfully committed blob to containerd content store.")
	}

	if desc.Size == 0 {
		desc.Size = bw.size
	}
	if desc.MediaType == "" {
		// Not sure if this is needed but the default registry blob writer assigns this.
		desc.MediaType = "application/octet-stream"
	}

	return desc, nil
}

// Cancel cancels the blob upload by deleting the containerd lease.
func (bw *blobWriter) Cancel(ctx context.Context) error {
	bw.log.Debug("Canceling upload: deleting containerd lease.")
	return bw.client.LeasesService().Delete(ctx, bw.lease)
}

// Close closes the containerd blob writer.
func (bw *blobWriter) Close() error {
	bw.log.Debug("Closing containerd blob writer.")
	err := bw.writer.Close()

	if bw.size == 0 {
		// It's safe to delete the lease if no data was written to the writer. Deletion is idempotent.
		err = errors.Join(bw.client.LeasesService().Delete(context.Background(), bw.lease))
	}

	return err
}
