package conformance

import (
	"fmt"
	"net/http"

	"strconv"

	"github.com/bloodorangeio/reggie"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var test02Push = func() {
	g.Context(titlePush, func() {

		var lastResponse, prevResponse *reggie.Response
		var emptyLayerManifestRef string

		g.Context("Setup", func() {
			// No setup required at this time for push tests
		})

		g.Context("Blob Upload Streamed", func() {
			g.Specify("PATCH request with blob in body should yield 202 response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())

				req = client.NewRequest(reggie.PATCH, resp.GetRelativeLocation()).
					SetHeader("Content-Type", "application/octet-stream").
					SetBody(testBlobA)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
				lastResponse = resp
			})

			g.Specify("PUT request to session URL with digest should yield 201 response", func() {
				SkipIfDisabled(push)
				Expect(lastResponse).ToNot(BeNil())
				req := client.NewRequest(reggie.PUT, lastResponse.GetRelativeLocation()).
					SetQueryParam("digest", testBlobADigest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testBlobALength)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
			})
		})

		g.Context("Blob Upload Monolithic", func() {
			g.Specify("GET nonexistent blob should result in 404 response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>",
					reggie.WithDigest(dummyDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("POST request with digest and blob should yield a 201 or 202", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/").
					SetHeader("Content-Length", configs[1].ContentLength).
					SetHeader("Content-Type", "application/octet-stream").
					SetQueryParam("digest", configs[1].Digest).
					SetBody(configs[1].Content)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusCreated),
					Equal(http.StatusAccepted),
				))
				lastResponse = resp
			})

			g.Specify("GET request to blob URL from prior request should yield 200 or 404 based on response code", func() {
				SkipIfDisabled(push)
				Expect(lastResponse).ToNot(BeNil())
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[1].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				if lastResponse.StatusCode() == http.StatusAccepted {
					Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
				} else {
					Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				}
			})

			g.Specify("POST request should yield a session ID", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
				lastResponse = resp
			})

			g.Specify("PUT upload of a blob should yield a 201 Response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.PUT, lastResponse.GetRelativeLocation()).
					SetHeader("Content-Length", configs[1].ContentLength).
					SetHeader("Content-Type", "application/octet-stream").
					SetQueryParam("digest", configs[1].Digest).
					SetBody(configs[1].Content)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
				Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			})

			g.Specify("GET request to existing blob should yield 200 response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[1].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})

			g.Specify("PUT upload of a layer blob should yield a 201 Response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetHeader("Content-Length", layerBlobContentLength).
					SetHeader("Content-Type", "application/octet-stream").
					SetQueryParam("digest", layerBlobDigest).
					SetBody(layerBlobData)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
				Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			})

			g.Specify("GET request to existing layer should yield 200 response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>", reggie.WithDigest(layerBlobDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})
		})

		g.Context("Blob Upload Chunked", func() {
			g.Specify("Out-of-order blob upload should return 416", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/").
					SetHeader("Content-Length", "0")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())

				// rebuild chunked blob if min size is above our chunk size
				minSizeStr := resp.Header().Get("OCI-Chunk-Min-Length")
				if minSizeStr != "" {
					minSize, err := strconv.Atoi(minSizeStr)
					Expect(err).To(BeNil())
					if minSize > len(testBlobBChunk1) {
						setupChunkedBlob(minSize*2 - 2)
					}
				}

				req = client.NewRequest(reggie.PATCH, resp.GetRelativeLocation()).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testBlobBChunk2Length).
					SetHeader("Content-Range", testBlobBChunk2Range).
					SetBody(testBlobBChunk2)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusRequestedRangeNotSatisfiable))
			})

			g.Specify("PATCH request with first chunk should return 202", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/").
					SetHeader("Content-Length", "0")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
				prevResponse = resp
				req = client.NewRequest(reggie.PATCH, resp.GetRelativeLocation()).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testBlobBChunk1Length).
					SetHeader("Content-Range", testBlobBChunk1Range).
					SetBody(testBlobBChunk1)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
				Expect(resp.Header().Get("Range")).To(Equal(testBlobBChunk1Range))
				lastResponse = resp
			})

			g.Specify("Retry previous blob chunk should return 416", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.PATCH, prevResponse.GetRelativeLocation()).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", testBlobBChunk1Length).
					SetHeader("Content-Range", testBlobBChunk1Range).
					SetBody(testBlobBChunk1)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusRequestedRangeNotSatisfiable))
			})

			g.Specify("Get on stale blob upload should return 204 with a range and location", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, prevResponse.GetRelativeLocation())
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNoContent))
				Expect(resp.Header().Get("Location")).ToNot(BeEmpty())
				Expect(resp.Header().Get("Range")).To(Equal(testBlobBChunk1Range))
				lastResponse = resp
			})

			g.Specify("PATCH request with second chunk should return 202", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.PATCH, lastResponse.GetRelativeLocation()).
					SetHeader("Content-Length", testBlobBChunk2Length).
					SetHeader("Content-Range", testBlobBChunk2Range).
					SetHeader("Content-Type", "application/octet-stream").
					SetBody(testBlobBChunk2)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				location := resp.Header().Get("Location")
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
				Expect(location).ToNot(BeEmpty())
				lastResponse = resp
			})

			g.Specify("PUT request with digest should return 201", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.PUT, lastResponse.GetRelativeLocation()).
					SetHeader("Content-Length", "0").
					SetHeader("Content-Type", "application/octet-stream").
					SetQueryParam("digest", testBlobBDigest)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
				location := resp.Header().Get("Location")
				Expect(location).ToNot(BeEmpty())
			})
		})

		g.Context("Cross-Repository Blob Mount", func() {
			g.Specify("Cross-mounting of a blob without the from argument should yield session id", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/",
					reggie.WithName(crossmountNamespace)).
					SetQueryParam("mount", dummyDigest)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
				Expect(resp.GetAbsoluteLocation()).To(Not(BeEmpty()))
			})

			g.Specify("POST request to mount another repository's blob should return 201 or 202", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/",
					reggie.WithName(crossmountNamespace)).
					SetQueryParam("mount", testBlobADigest).
					SetQueryParam("from", client.Config.DefaultName)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusCreated),
					Equal(http.StatusAccepted),
				))
				lastResponse = resp
			})

			g.Specify("GET request to test digest within cross-mount namespace should return 200", func() {
				SkipIfDisabled(push)
				RunOnlyIf(lastResponse.StatusCode() == http.StatusCreated)
				Expect(lastResponse.GetRelativeLocation()).To(Equal(fmt.Sprintf("/v2/%s/blobs/%s", crossmountNamespace, testBlobADigest)))
				req := client.NewRequest(reggie.GET, lastResponse.GetRelativeLocation())
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})

			g.Specify("Cross-mounting of nonexistent blob should yield session id", func() {
				SkipIfDisabled(push)
				RunOnlyIf(lastResponse.StatusCode() == http.StatusAccepted)
				Expect(lastResponse.GetRelativeLocation()).To(HavePrefix(fmt.Sprintf("/v2/%s/blobs/uploads/", crossmountNamespace)))
			})

			g.Specify("Cross-mounting without from, and automatic content discovery enabled should return a 201", func() {
				SkipIfDisabled(push)
				RunOnlyIf(runAutomaticCrossmountTest)
				RunOnlyIf(lastResponse.StatusCode() == http.StatusCreated)
				RunOnlyIf(automaticCrossmountEnabled)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/",
					reggie.WithName(crossmountNamespace)).
					SetQueryParam("mount", testBlobADigest)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
			})

			g.Specify("Cross-mounting without from, and automatic content discovery disabled should return a 202", func() {
				SkipIfDisabled(push)
				RunOnlyIf(runAutomaticCrossmountTest)
				RunOnlyIf(lastResponse.StatusCode() == http.StatusCreated)
				RunOnlyIfNot(automaticCrossmountEnabled)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/",
					reggie.WithName(crossmountNamespace)).
					SetQueryParam("mount", testBlobADigest)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusAccepted))
			})
		})

		g.Context("Manifest Upload", func() {
			g.Specify("GET nonexistent manifest should return 404", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(nonexistentManifest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("PUT should accept a manifest upload", func() {
				SkipIfDisabled(push)
				for i := 0; i < 4; i++ {
					tag := fmt.Sprintf("test%d", i)
					req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
						reggie.WithReference(tag)).
						SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
						SetBody(manifests[1].Content)
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					location := resp.Header().Get("Location")
					Expect(location).ToNot(BeEmpty())
					Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
				}
			})

			g.Specify("Registry should accept a manifest upload with no layers", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(emptyLayerTestTag)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(emptyLayerManifestContent)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				if resp.StatusCode() == http.StatusCreated {
					location := resp.Header().Get("Location")
					emptyLayerManifestRef = location
					Expect(location).ToNot(BeEmpty())
					Expect(resp.StatusCode()).To(Equal(http.StatusCreated))
				} else {
					Warn("image manifest with no layers is not supported")
				}
			})

			g.Specify("GET request to manifest URL (digest) should yield 200 response", func() {
				SkipIfDisabled(push)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[1].Digest)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})
		})

		g.Context("Teardown", func() {
			if deleteManifestBeforeBlobs {
				g.Specify("Delete manifest created in tests", func() {
					SkipIfDisabled(push)
					RunOnlyIf(runPushSetup)
					req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[1].Digest))
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAny(
						SatisfyAll(
							BeNumerically(">=", 200),
							BeNumerically("<", 300),
						),
						Equal(http.StatusMethodNotAllowed),
					))
					if emptyLayerManifestRef != "" {
						req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>", reggie.WithReference(emptyLayerManifestDigest))
						resp, err = client.Do(req)
						Expect(err).To(BeNil())
						Expect(resp.StatusCode()).To(SatisfyAny(
							SatisfyAll(
								BeNumerically(">=", 200),
								BeNumerically("<", 300),
							),
							Equal(http.StatusMethodNotAllowed),
						))
					}
				})
			}

			g.Specify("Delete config blob created in tests", func() {
				SkipIfDisabled(push)
				RunOnlyIf(runPushSetup)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[1].Digest))
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
				SkipIfDisabled(push)
				RunOnlyIf(runPushSetup)
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
				g.Specify("Delete manifest created in tests", func() {
					SkipIfDisabled(push)
					RunOnlyIf(runPushSetup)
					req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[1].Digest))
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAny(
						SatisfyAll(
							BeNumerically(">=", 200),
							BeNumerically("<", 300),
						),
						Equal(http.StatusMethodNotAllowed),
					))
					if emptyLayerManifestRef != "" {
						req = client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>", reggie.WithReference(emptyLayerManifestDigest))
						resp, err = client.Do(req)
						Expect(err).To(BeNil())
						Expect(resp.StatusCode()).To(SatisfyAny(
							SatisfyAll(
								BeNumerically(">=", 200),
								BeNumerically("<", 300),
							),
							Equal(http.StatusMethodNotAllowed),
						))
					}
				})
			}
		})
	})
}
