package containerd

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/errdefs"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
)

// tagService implements distribution.TagService backed by the containerd image store.
type tagService struct {
	client *client.Client
	// canonicalRepo is the repository reference in a normalized form, the way containerd image store expects it,
	// for example, "docker.io/library/ubuntu"
	canonicalRepo reference.Named
}

// Get retrieves an image descriptor by its tag from the containerd image store.
func (t *tagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	ref, err := reference.WithTag(t.canonicalRepo, tag)
	if err != nil {
		return distribution.Descriptor{}, distribution.ErrManifestUnknown{
			Name: t.canonicalRepo.Name(),
			Tag:  tag,
		}
	}

	img, err := t.client.ImageService().Get(ctx, ref.String())
	if err != nil {
		logrus.WithField("image", ref.String()).WithError(err).Debug("Failed to get image from containerd image store.")
		if errdefs.IsNotFound(err) {
			return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}

		}
		return distribution.Descriptor{}, fmt.Errorf(
			"get image '%s' from containerd image store: %w", ref.String(), err,
		)
	}
	logrus.WithFields(
		logrus.Fields{
			"image":      ref.String(),
			"descriptor": img.Target,
		},
	).Debug("Got image from containerd image store.")

	return img.Target, nil
}

// Tag creates or updates the image tag in the containerd image store. The descriptor must be an image/index manifest
// that is already present in the containerd content store.
// It also sets garbage collection labels on the image content in the containerd content store to prevent it from being
// deleted by garbage collection.
func (t *tagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	ref, err := reference.WithTag(t.canonicalRepo, tag)
	if err != nil {
		return err
	}

	img := images.Image{
		Name:   ref.String(),
		Target: desc,
	}

	// Just before creating or updating the image in the containerd image store, we need to assign appropriate garbage
	// collection labels to its content (manifests, config, layers). This is necessary to ensure that the content is not
	// deleted by GC once the leases that uploaded the content are expired or deleted.
	// See for more details:
	// https://github.com/containerd/containerd/blob/main/docs/garbage-collection.md#garbage-collection-labels
	//
	// TODO: delete unnecessary leases after setting the GC labels. It seems to be non-trivial to do so, because we need
	//  to keep track of which leases were used to upload which content and share this info between
	//  the blobStore/blobWriter and tagService. The downside of keeping them around is the image content will be kept
	//  in the store even if the image is deleted, until the leases expire (default is leaseExpiration).

	contentStore := t.client.ContentStore()
	// Get all the children descriptors (manifests, config, layers) for an image index or manifest.
	childrenHandler := images.ChildrenHandler(contentStore)
	// Recursively set garbage collection labels on each descriptor for the content of its children to prevent them
	// from being deleted by GC.
	setGCLabelsHandler := images.SetChildrenMappedLabels(contentStore, childrenHandler, nil)
	if err = images.Dispatch(ctx, setGCLabelsHandler, nil, desc); err != nil {
		return fmt.Errorf(
			"set garbage collection labels for content of image '%s' in containerd content store: %w", ref.String(),
			err,
		)
	}
	log := logrus.WithFields(
		logrus.Fields{
			"image":      ref.String(),
			"descriptor": desc,
		},
	)
	log.Debug("Set garbage collection labels for image content in containerd content store.")

	imageService := t.client.ImageService()
	if _, err = imageService.Create(ctx, img); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return fmt.Errorf("create image '%s' in containerd image store: %w", ref.String(), err)
		}

		_, err = imageService.Update(ctx, img)
		if err != nil {
			return fmt.Errorf("update image '%s' in containerd image store: %w", ref.String(), err)
		}

		log.Debug("Updated existing image in containerd image store.")
	} else {
		log.Debug("Created new image in containerd image store.")
	}

	return nil
}

// Untag is not supported for simplicity.
// An image could be untagged by deleting the image in containerd.
func (t *tagService) Untag(ctx context.Context, tag string) error {
	return distribution.ErrUnsupported
}

// All should return all tags associated with the repository but discovery operations are not supported for simplicity.
func (t *tagService) All(ctx context.Context) ([]string, error) {
	return nil, distribution.ErrUnsupported
}

// Lookup should find tags associated with a descriptor but discovery operations are not supported for simplicity.
func (t *tagService) Lookup(ctx context.Context, desc distribution.Descriptor) ([]string, error) {
	return nil, distribution.ErrUnsupported
}
