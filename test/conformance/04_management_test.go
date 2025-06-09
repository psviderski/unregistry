package conformance

import (
	"encoding/json"
	"net/http"

	"github.com/bloodorangeio/reggie"
	g "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var test04ContentManagement = func() {
	g.Context(titleContentManagement, func() {

		const defaultTagName = "tagtest0"
		var tagToDelete string
		var numTags int
		var blobDeleteAllowed = true

		g.Context("Setup", func() {
			g.Specify("Populate registry with test config blob", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
				req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
					SetHeader("Content-Length", configs[3].ContentLength).
					SetHeader("Content-Type", "application/octet-stream").
					SetQueryParam("digest", configs[3].Digest).
					SetBody(configs[3].Content)
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Populate registry with test layer", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
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

			g.Specify("Populate registry with test tag", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
				tagToDelete = defaultTagName
				req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(tagToDelete)).
					SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
					SetBody(manifests[3].Content)
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAll(
					BeNumerically(">=", 200),
					BeNumerically("<", 300)))
			})

			g.Specify("Check how many tags there are before anything gets deleted", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusOK))
				tagList := &TagList{}
				jsonData := []byte(resp.String())
				err = json.Unmarshal(jsonData, tagList)
				Expect(err).To(BeNil())
				numTags = len(tagList.Tags)
			})
		})

		g.Context("Manifest delete", func() {
			g.Specify("DELETE request to manifest tag should return 202, unless tag deletion is disallowed (400/405)", func() {
				SkipIfDisabled(contentManagement)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<reference>",
					reggie.WithReference(tagToDelete))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusBadRequest),
					Equal(http.StatusAccepted),
					Equal(http.StatusMethodNotAllowed)))
				if resp.StatusCode() == http.StatusBadRequest {
					errorResponses, err := resp.Errors()
					Expect(err).To(BeNil())
					Expect(errorResponses).ToNot(BeEmpty())
					Expect(errorResponses[0].Code).To(Equal(errorCodes[UNSUPPORTED]))
				}
			})

			g.Specify("DELETE request to manifest (digest) should yield 202 response unless already deleted", func() {
				SkipIfDisabled(contentManagement)
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[3].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				// In the case that the previous request was accepted, this may or may not fail (which is ok)
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusAccepted),
					Equal(http.StatusNotFound),
				))
			})

			g.Specify("GET request to deleted manifest URL should yield 404 response, unless delete is disallowed", func() {
				SkipIfDisabled(contentManagement)
				req := client.NewRequest(reggie.GET, "/v2/<name>/manifests/<digest>", reggie.WithDigest(manifests[3].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusNotFound),
					Equal(http.StatusOK),
				))
			})

			g.Specify("GET request to tags list should reflect manifest deletion", func() {
				SkipIfDisabled(contentManagement)
				req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list")
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusNotFound),
					Equal(http.StatusOK),
				))
				expectTags := numTags - 1
				if resp.StatusCode() == http.StatusOK {
					tagList := &TagList{}
					jsonData := []byte(resp.String())
					err = json.Unmarshal(jsonData, tagList)
					Expect(err).To(BeNil())
					Expect(len(tagList.Tags)).To(Equal(expectTags))
				}
				if resp.StatusCode() == http.StatusNotFound {
					Expect(expectTags).To(Equal(0))
				}
			})
		})

		g.Context("Blob delete", func() {
			g.Specify("DELETE request to blob URL should yield 202 response", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
				// config blob
				req := client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[3].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusAccepted),
					Equal(http.StatusNotFound),
					Equal(http.StatusMethodNotAllowed),
				))
				// layer blob
				req = client.NewRequest(reggie.DELETE, "/v2/<name>/blobs/<digest>", reggie.WithDigest(layerBlobDigest))
				resp, err = client.Do(req)
				Expect(err).To(BeNil())
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(SatisfyAny(
					Equal(http.StatusAccepted),
					Equal(http.StatusNotFound),
					Equal(http.StatusMethodNotAllowed),
				))
				if resp.StatusCode() == http.StatusMethodNotAllowed {
					blobDeleteAllowed = false
				}
			})

			g.Specify("GET request to deleted blob URL should yield 404 response", func() {
				SkipIfDisabled(contentManagement)
				RunOnlyIf(runContentManagementSetup)
				RunOnlyIf(blobDeleteAllowed)
				req := client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>", reggie.WithDigest(configs[3].Digest))
				resp, err := client.Do(req)
				Expect(err).To(BeNil())
				Expect(resp.StatusCode()).To(Equal(http.StatusNotFound))
			})
		})

		g.Context("Teardown", func() {
			// TODO: delete blob+tag?
			// No teardown required at this time for content management tests
		})
	})
}
