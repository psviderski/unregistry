package conformance

import (
	"net/http"
	"os"

	"github.com/bloodorangeio/reggie"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var test01Pull = func() {
	g.Context(titlePull, func() {

		var tag string

		g.Context("Setup", func() {
			g.Specify("Populate registry with test blob", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", configs[0].Digest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", configs[0].ContentLength).
					SetBody(configs[0].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test blob", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetQueryParam("digest", configs[1].Digest).
					SetHeader("Content-Type", "application/octet-stream").
					SetHeader("Content-Length", configs[1].ContentLength).
					SetBody(configs[1].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test layer", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
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

			g.Specify("Populate registry with test manifest", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
				tag = testTagName
				req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(tag)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(manifests[0].Content)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test manifest", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
				req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(manifests[1].Digest)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(manifests[1].Content)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Get tag name from environment", func() {
				SkipIfDisabled(pull)
				RunOnlyIfNot(runPullSetup)
				tmp := os.Getenv(envVarTagName)
				if tmp != "" {
					tag = tmp
				}
			})
		})

		g.Context("Pull blobs", func() {
			g.Specify("HEAD request to nonexistent blob should result in 404 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/blobs/<digest>",
					reggie.WithDigest(dummyDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("HEAD request to existing blob should yield 200", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/blobs/<digest>",
					reggie.WithDigest(configs[0].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				if h := resp.Header().Get("Docker-Content-Digest"); h != "" {
					Expect(h).To(Equal(configs[0].Digest))
				}
			})

			g.Specify("GET nonexistent blob should result in 404 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>",
					reggie.WithDigest(dummyDigest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("GET request to existing blob URL should yield 200", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[0].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})
		})

		g.Context("Pull manifests", func() {
			g.Specify("HEAD request to nonexistent manifest should return 404", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(nonexistentManifest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("HEAD request to manifest[0] path (digest) should yield 200 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[0].Digest)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				if h := resp.Header().Get("Docker-Content-Digest"); h != "" {
					Expect(h).To(Equal(manifests[0].Digest))
				}
			})

			g.Specify("HEAD request to manifest[1] path (digest) should yield 200 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[1].Digest)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				if h := resp.Header().Get("Docker-Content-Digest"); h != "" {
					Expect(h).To(Equal(manifests[1].Digest))
				}
			})

			g.Specify("HEAD request to manifest path (tag) should yield 200 response", func() {
				SkipIfDisabled(pull)
				Expect(tag).ToNot(BeEmpty())
				req := client.NewRequest(reggie.HEAD, "/v2/<name>/manifests/<reference>", reggie.WithReference(tag)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				if h := resp.Header().Get("Docker-Content-Digest"); h != "" {
					Expect(h).To(Equal(manifests[0].Digest))
				}
			})

			g.Specify("GET nonexistent manifest should return 404", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(nonexistentManifest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})

			g.Specify("GET request to manifest[0] path (digest) should yield 200 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[0].Digest)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})

			g.Specify("GET request to manifest[1] path (digest) should yield 200 response", func() {
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[1].Digest)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})

			g.Specify("GET request to manifest path (tag) should yield 200 response", func() {
				SkipIfDisabled(pull)
				Expect(tag).ToNot(BeEmpty())
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>", reggie.WithReference(tag)).
					SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
			})
		})

		g.Context("Error codes", func() {
			g.Specify("400 response body should contain OCI-conforming JSON message", func() {
				g.Skip("Skipped as the distribution package returns 500 for invalid manifests")
				SkipIfDisabled(pull)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>",
					reggie.WithReference("sha256:totallywrong")).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(invalidManifestContent)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusBadRequest),
					Equal(http.StatusNotFound)))
				if resp.StatusCode() == http.StatusBadRequest {
					errorResponses, err := resp.Errors()
					Expect(err).To(BeNil())

					Expect(errorResponses).ToNot(BeEmpty())
					Expect(errorCodes).To(ContainElement(errorResponses[0].Code))
				}
			})
		})

		g.Context("Teardown", func() {
			if deleteManifestBeforeBlobs {
				g.Specify("Delete manifest[0] created in setup", func() {
					SkipIfDisabled(pull)
					RunOnlyIf(runPullSetup)
					req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[0].Digest))
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAny(
						SatisfyAll(
							BeNumerically(">=", 200),
							BeNumerically("<", 300),
						),
						Equal(http.StatusMethodNotAllowed),
					))
				})
				g.Specify("Delete manifest[1] created in setup", func() {
					SkipIfDisabled(pull)
					RunOnlyIf(runPullSetup)
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
				})
			}

			g.Specify("Delete config[0] blob created in setup", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[0].Digest))
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
			g.Specify("Delete config[1] blob created in setup", func() {
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
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
				SkipIfDisabled(pull)
				RunOnlyIf(runPullSetup)
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
				g.Specify("Delete manifest[0] created in setup", func() {
					SkipIfDisabled(pull)
					RunOnlyIf(runPullSetup)
					req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[0].Digest))
					resp, err := client.Do(req)
					Expect(err).To(BeNil())
					Expect(resp.StatusCode()).To(SatisfyAny(
						SatisfyAll(
							BeNumerically(">=", 200),
							BeNumerically("<", 300),
						),
						Equal(http.StatusMethodNotAllowed),
					))
				})
				g.Specify("Delete manifest[1] created in setup", func() {
					SkipIfDisabled(pull)
					RunOnlyIf(runPullSetup)
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
				})
			}
		})
	})
}
