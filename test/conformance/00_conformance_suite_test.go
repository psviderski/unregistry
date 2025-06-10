package conformance

import (
	"log"
	"os"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
)

func TestConformance(t *testing.T) {
	// Setup unregistry container before running tests.
	unregistryContainer, url := SetupUnregistry(t)
	// Clean up the container after all tests.
	t.Cleanup(func() {
		TeardownUnregistry(t, unregistryContainer)
	})

	// Configure environment variables for conformance tests.
	os.Setenv("OCI_ROOT_URL", url)
	os.Setenv("OCI_NAMESPACE", "conformance")
	// Enable push and pull tests only. Discover and management are not supported yet.
	os.Setenv("OCI_TEST_PULL", "1")
	os.Setenv("OCI_TEST_PUSH", "1")
	//os.Setenv("OCI_TEST_CONTENT_DISCOVERY", "1")
	//os.Setenv("OCI_TEST_CONTENT_MANAGEMENT", "1")
	// Set debug mode for better logging.
	//os.Setenv("OCI_DEBUG", "1")

	setup()

	g.Describe(suiteDescription, func() {
		test01Pull()
		test02Push()
		test03ContentDiscovery()
		test04ContentManagement()
	})

	RegisterFailHandler(g.Fail)
	suiteConfig, reporterConfig := g.GinkgoConfiguration()
	hr := newHTMLReporter(reportHTMLFilename)
	g.ReportAfterEach(hr.afterReport)
	g.ReportAfterSuite("html custom reporter", func(r g.Report) {
		if err := hr.endSuite(r); err != nil {
			log.Printf("\nWARNING: cannot write HTML summary report: %v", err)
		}
	})
	g.ReportAfterSuite("junit custom reporter", func(r g.Report) {
		if reportJUnitFilename != "" {
			_ = reporters.GenerateJUnitReportWithConfig(r, reportJUnitFilename, reporters.JunitReportConfig{
				OmitLeafNodeType: true,
			})
		}
	})
	g.RunSpecs(t, "conformance tests", suiteConfig, reporterConfig)
}
