package containerd

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/errdefs"
	"github.com/distribution/distribution/v3"
	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// tagService implements distribution.TagService backed by containerd image store.
type tagService struct {
	client *Client
	repo   reference.Named
}

// Get retrieves an image descriptor by its tag from the containerd image store.
func (t *tagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	// Construct the full reference
	ref := fmt.Sprintf("%s:%s", t.repo.String(), tag)

	// Get the image from containerd
	img, err := t.client.ImageStore().Get(ctx, ref)
	if err != nil {
		// Log for debugging
		fmt.Printf("DEBUG: tagService.Get failed for ref %s: %v\n", ref, err)
		if errdefs.IsNotFound(err) {
			return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}

		}
		return distribution.Descriptor{}, err
	}

	return distribution.Descriptor{
		MediaType: img.Target.MediaType,
		Digest:    img.Target.Digest,
		Size:      img.Target.Size,
	}, nil
}

// Tag associates a tag with a descriptor.
func (t *tagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	ctx = t.client.Context(ctx)

	// Construct the full reference
	ref := fmt.Sprintf("%s:%s", t.repo.String(), tag)

	// Create or update the image
	img := images.Image{
		Name: ref,
		Target: ocispec.Descriptor{
			MediaType: desc.MediaType,
			Digest:    desc.Digest,
			Size:      desc.Size,
		},
	}

	// Try to update first
	_, err := t.client.ImageStore().Update(ctx, img)
	if err != nil {
		// If update fails, try to create
		_, err = t.client.ImageStore().Create(ctx, img)
		if err != nil {
			return err
		}
	}

	return nil
}

// Untag removes a tag.
func (t *tagService) Untag(ctx context.Context, tag string) error {
	ctx = t.client.Context(ctx)

	// Construct the full reference
	ref := fmt.Sprintf("%s:%s", t.repo.String(), tag)

	// Delete the image reference
	err := t.client.ImageStore().Delete(ctx, ref)
	if err != nil {
		return convertError(err)
	}

	return nil
}

// All returns all tags for the repository.
func (t *tagService) All(ctx context.Context) ([]string, error) {
	ctx = t.client.Context(ctx)

	// List all images
	images, err := t.client.ImageStore().List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by repository name
	repoName := t.repo.String()
	var tags []string

	for _, img := range images {
		// Check if the image belongs to this repository
		if strings.HasPrefix(img.Name, repoName+":") {
			tag := strings.TrimPrefix(img.Name, repoName+":")
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

// Lookup finds tags associated with a descriptor.
func (t *tagService) Lookup(ctx context.Context, desc distribution.Descriptor) ([]string, error) {
	ctx = t.client.Context(ctx)

	// List all images
	images, err := t.client.ImageStore().List(ctx)
	if err != nil {
		return nil, err
	}

	// Find tags that point to this descriptor
	repoName := t.repo.String()
	var tags []string

	for _, img := range images {
		// Check if the image belongs to this repository and has the same digest
		if strings.HasPrefix(img.Name, repoName+":") && img.Target.Digest == desc.Digest {
			tag := strings.TrimPrefix(img.Name, repoName+":")
			tags = append(tags, tag)
		}
	}

	return tags, nil
}
