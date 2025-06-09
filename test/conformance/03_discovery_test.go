package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/bloodorangeio/reggie"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	godigest "github.com/opencontainers/go-digest"
)

var test03ContentDiscovery = func() {
	g.Context(titleContentDiscovery, func() {

		var numTags = 4
		var tagList []string

		g.Context("Setup", func() {
			g.Specify("Populate registry with test blob", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", configs[2].Digest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", configs[2].ContentLength).
					SetBody(configs[2].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test layer", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", layerBlobDigest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", layerBlobContentLength).
					SetBody(layerBlobData)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test tags", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)
				for i := 0; i < numTags; i++ {
					tag := fmt.Sprintf("test%d", i)
					tagList = append(tagList, tag)
					req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(tag)).
						SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
						SetBody(manifests[2].Content)
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAll(
						BeNumerically(">=", 200),
						BeNumerically("<", 300)))
				}
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list")
				resp, err := client.Do(req)
				tagList = getTagList(resp)
				_ = err
			})

			g.Specify("Populate registry with test tags (no push)", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIfNot(runContentDiscoverySetup)
				tagList = strings.Split(os.Getenv(envVarTagList), ",")
			})

			g.Specify("References setup", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)

				// Populate registry with empty JSON blob
				// validate expected empty JSON blob digest
				Expect(emptyJSONDescriptor.Digest).To(Equal(godigest.Digest("sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a")))
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", emptyJSONDescriptor.Digest.String()).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", fmt.Sprintf("%d", emptyJSONDescriptor.Size)).
					SetBody(emptyJSONBlob)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with reference blob before the image manifest is pushed
				req = client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", testRefBlobADigest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testRefBlobALength).
					SetBody(testRefBlobA)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with test references manifest (config.MediaType = artifactType)
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsManifestAConfigArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(refsManifestAConfigArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[4].Digest))

				// Populate registry with test references manifest (ArtifactType, config.MediaType = emptyJSON)
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsManifestALayerArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(refsManifestALayerArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[4].Digest))

				// Populate registry with test index manifest
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsIndexArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.index.v1+json").
					SetBody(refsIndexArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[4].Digest))

				// Populate registry with test blob
				req = client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", configs[4].Digest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", configs[4].ContentLength).
					SetBody(configs[4].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with test layer
				req = client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", layerBlobDigest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", layerBlobContentLength).
					SetBody(layerBlobData)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with test manifest
				tag := testTagName
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(tag)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(manifests[4].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with reference blob after the image manifest is pushed
				req = client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", testRefBlobBDigest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testRefBlobBLength).
					SetBody(testRefBlobB)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))

				// Populate registry with test references manifest (config.MediaType = artifactType)
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsManifestBConfigArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(refsManifestBConfigArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[4].Digest))

				// Populate registry with test references manifest (ArtifactType, config.MediaType = emptyJSON)
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsManifestBLayerArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(refsManifestBLayerArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[4].Digest))

				// Populate registry with test references manifest to a non-existent subject
				req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(refsManifestCLayerArtifactDigest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(refsManifestCLayerArtifactContent)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
				Expect(resp.Header().Get("OCI-Subject")).To(Equal(manifests[3].Digest))
			})
		})

		g.Context("Test content discovery endpoints (listing tags)", func() {
			g.Specify("GET request to list tags should yield 200 response", func() {
				SkipIfDisabled(contentDiscovery)
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				tagList = getTagList(resp)
				numTags = len(tagList)
			})

			g.Specify("GET number of tags should be limitable by `n` query parameter", func() {
				SkipIfDisabled(contentDiscovery)
				numResults := numTags / 2
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list").
					SetQueryParam("n", strconv.Itoa(numResults))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				tagList = getTagList(resp)
				Expect(err).To(BeNil())
				Expect(len(tagList)).To(Equal(numResults))
			})

			g.Specify("GET start of tag is set by `last` query parameter", func() {
				SkipIfDisabled(contentDiscovery)
				numResults := numTags / 2
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list").
					SetQueryParam("n", strconv.Itoa(numResults))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				tagList = getTagList(resp)
				req = client.NewRequest(reggie.GET, "/v2/<name>/tags/list").
					SetQueryParam("n", strconv.Itoa(numResults)).
					SetQueryParam("last", tagList[numResults-1])
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(err).To(BeNil())
				Expect(len(tagList)).To(BeNumerically("<=", numResults))
				Expect(tagList).To(ContainElement(tagList[numResults-1]))
			})
		})

		g.Context("Test content discovery endpoints (listing references)", func() {
			g.Specify("GET request to nonexistent blob should result in empty 200 response", func() {
				SkipIfDisabled(contentDiscovery)
				req := client.NewRequest(reggie.GET, "/v2/<name>/referrers/<digest>",
					reggie.WithDigest(dummyDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.Header().Get("Content-Type")).To(Equal("application/vnd.oci.image.index.v1+json"))

				var index index
				err = json.Unmarshal(resp.Body(), &index)
				Expect(err).To(BeNil())
				Expect(len(index.Manifests)).To(BeZero())
			})

			g.Specify("GET request to existing blob should yield 200", func() {
				SkipIfDisabled(contentDiscovery)
				req := client.NewRequest(reggie.GET, "/v2/<name>/referrers/<digest>",
					reggie.WithDigest(manifests[4].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.Header().Get("Content-Type")).To(Equal("application/vnd.oci.image.index.v1+json"))

				var index index
				err = json.Unmarshal(resp.Body(), &index)
				Expect(err).To(BeNil())
				Expect(len(index.Manifests)).To(Equal(5))
				Expect(index.Manifests[0].Digest).ToNot(Equal(index.Manifests[1].Digest))
				for i := 0; i < len(index.Manifests); i++ {
					Expect(len(index.Manifests[i].Annotations)).To(Equal(1))
					Expect(index.Manifests[i].Annotations[testAnnotationKey]).To(Equal(testAnnotationValues[index.Manifests[i].Digest.String()]))
				}
			})

			g.Specify("GET request to existing blob with filter should yield 200", func() {
				SkipIfDisabled(contentDiscovery)
				req := client.NewRequest(reggie.GET, "/v2/<name>/referrers/<digest>",
					reggie.WithDigest(manifests[4].Digest)).
					SetQueryParam("artifactType", testRefArtifactTypeA)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.Header().Get("Content-Type")).To(Equal("application/vnd.oci.image.index.v1+json"))

				var index index
				err = json.Unmarshal(resp.Body(), &index)
				Expect(err).To(BeNil())

				// also check resp header "OCI-Filters-Applied: artifactType" denoting that an artifactType filter was applied
				if resp.Header().Get("OCI-Filters-Applied") != "" {
					Expect(len(index.Manifests)).To(Equal(2))
					Expect(resp.Header().Get("OCI-Filters-Applied")).To(Equal(artifactTypeFilter))
					for i := 0; i < len(index.Manifests); i++ {
						Expect(len(index.Manifests[i].Annotations)).To(Equal(1))
						Expect(index.Manifests[i].Annotations[testAnnotationKey]).To(Equal(testAnnotationValues[index.Manifests[i].Digest.String()]))
					}
				} else {
					Expect(len(index.Manifests)).To(Equal(5))
					for i := 0; i < len(index.Manifests); i++ {
						Expect(len(index.Manifests[i].Annotations)).To(Equal(1))
						Expect(index.Manifests[i].Annotations[testAnnotationKey]).To(Equal(testAnnotationValues[index.Manifests[i].Digest.String()]))
					}
					Warn("filtering by artifact-type is not implemented")
				}
			})

			g.Specify("GET request to missing manifest should yield 200", func() {
				SkipIfDisabled(contentDiscovery)
				req := client.NewRequest(reggie.GET, "/v2/<name>/referrers/<digest>",
					reggie.WithDigest(manifests[3].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				Expect(resp.Header().Get("Content-Type")).To(Equal("application/vnd.oci.image.index.v1+json"))

				var index index
				err = json.Unmarshal(resp.Body(), &index)
				Expect(err).To(BeNil())
				Expect(len(index.Manifests)).To(Equal(1))
				Expect(index.Manifests[0].Digest.String()).To(Equal(refsManifestCLayerArtifactDigest))
			})
		})

		g.Context("Teardown", func() {
			if deleteManifestBeforeBlobs {
				g.Specify("Delete created manifest & associated tags", func() {
					SkipIfDisabled(contentDiscovery)
					RunOnlyIf(runContentDiscoverySetup)
					references := []string{
						refsIndexArtifactDigest,
						manifests[2].Digest,
						manifests[4].Digest,
						refsManifestAConfigArtifactDigest,
						refsManifestALayerArtifactDigest,
						refsManifestBConfigArtifactDigest,
						refsManifestBLayerArtifactDigest,
						refsManifestCLayerArtifactDigest,
					}
					for _, ref := range references {
						req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(ref))
						resp, err := client.Do(req)
						Expect(err).To(BeNil())
						Expect(resp.StatusCode()).To(SatisfyAny(
							SatisfyAll(
								BeNumerically(">=", 200),
								BeNumerically("<", 300),
							),
							Equal(http.StatusNotFound),
							Equal(http.StatusMethodNotAllowed),
						))
					}
				})
			}

			g.Specify("Delete config blob created in tests", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[2].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					SatisfyAll(
						BeNumerically(">=", 200),
						BeNumerically("<", 300),
					),
					Equal(http.StatusNotFound),
					Equal(http.StatusMethodNotAllowed),
				))
			})

			g.Specify("Delete layer blob created in setup", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(layerBlobDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					SatisfyAll(
						BeNumerically(">=", 200),
						BeNumerically("<", 300),
					),
					Equal(http.StatusNotFound),
					Equal(http.StatusMethodNotAllowed),
				))
			})

			if !deleteManifestBeforeBlobs {
				g.Specify("Delete created manifest & associated tags", func() {
					SkipIfDisabled(contentDiscovery)
					RunOnlyIf(runContentDiscoverySetup)
					references := []string{
						refsIndexArtifactDigest,
						manifests[2].Digest,
						manifests[4].Digest,
						refsManifestAConfigArtifactDigest,
						refsManifestALayerArtifactDigest,
						refsManifestBConfigArtifactDigest,
						refsManifestBLayerArtifactDigest,
						refsManifestCLayerArtifactDigest,
					}
					for _, ref := range references {
						req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(ref))
						resp, err := client.Do(req)
						Expect(err).To(BeNil())
						Expect(resp.StatusCode()).To(SatisfyAny(
							SatisfyAll(
								BeNumerically(">=", 200),
								BeNumerically("<", 300),
							),
							Equal(http.StatusMethodNotAllowed),
							Equal(http.StatusNotFound),
						))
					}
				})
			}

			g.Specify("References teardown", func() {
				SkipIfDisabled(contentDiscovery)
				RunOnlyIf(runContentDiscoverySetup)

				deleteReq := func(req *reggie.Request) {
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAny(
						SatisfyAll(
							BeNumerically(">=", 200),
							BeNumerically("<", 300),
						),
						Equal(http.StatusNotFound),
						Equal(http.StatusMethodNotAllowed),
					))
				}

				if deleteManifestBeforeBlobs {
					req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsIndexArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestAConfigArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestALayerArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[4].Digest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestBConfigArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestBLayerArtifactDigest))
					deleteReq(req)
				}

				// Delete config blob created in setup
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[4].Digest))
				deleteReq(req)

				// Delete reference blob created in setup
				req = client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(testRefBlobADigest))
				deleteReq(req)
				req = client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(testRefBlobBDigest))
				deleteReq(req)

				// Delete empty JSON blob created in setup
				req = client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(emptyJSONDescriptor.Digest.String()))
				deleteReq(req)

				if !deleteManifestBeforeBlobs {
					// Delete manifest created in setup
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsIndexArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestAConfigArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestALayerArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[4].Digest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestBConfigArtifactDigest))
					deleteReq(req)
					req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(refsManifestBLayerArtifactDigest))
					deleteReq(req)
				}
			})
		})
	})
}
