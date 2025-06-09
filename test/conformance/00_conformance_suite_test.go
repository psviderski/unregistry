package conformance

import (
	"log"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
)

func TestConformance(t *testing.T) {
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
