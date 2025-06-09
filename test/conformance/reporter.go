package conformance

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2/types"
)

const (
	suiteIndex           = 1
	categoryIndex        = 2
	setupString          = "Setup"
	htmlTemplate  string = `<html>
  <head>
    <title>OCI Distribution Conformance Tests</title>
    <style>
      body {
        padding: 10px 20px 10px 20px;
        font-family: -apple-system,BlinkMacSystemFont,Segoe UI,PingFang SC,Hiragino Sans GB,Microsoft YaHei,Helvetica Neue,Helvetica,Arial,sans-serif,Apple Color Emoji,Segoe UI Emoji,Segoe UI Symbol;
        background: url("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAQAAAAECAYAAACp8Z5+AAAAG0lEQVQYV2Pce7zwv7NlPyMDFMAZGAIwlRgqAFydCAVv5m4UAAAAAElFTkSuQmCC") repeat;
        /* background made with http://www.patternify.com/ */
      }
      table {
        border-collapse: collapse;
        width: 100%;
        background-color: white;
      }
      th, td {
        padding: 12px;
        text-align: left;
        border-bottom: 1px solid #ddd;
      }
      tr:hover {
        background-color: #ffe39b;
      }
      .result {
        padding: 1.25em 0 .25em 0.8em;
        border: 1px solid #e1e1e1;
        border-radius: 5px;
        margin-top: 10px;
      }
      .red {
        background: #ffc8c8;
      }
      pre.fail-message {
        background: #f9a5a5;
        padding: 20px;
        margin-right: 10px;
        display: inline-block;
        border-radius: 4px;
        font-size: 1.25em;
        width: 94%;
        overflow-x: auto;
        max-width: 85%;
      }
      .green {
        background: #c8ffc8;
        padding: 1.25em 0 1.25em 0.8em;
      }
      .grey {
        background: lightgrey;
        padding: 1.25em 0 1.25em 0.8em;
      }
      .toggle {
        border: 2px solid #3e3e3e;
        cursor: pointer;
        width: 1em;
        text-align: center;
        font-weight: bold;
        display: inline;
        font-family: monospace;
        padding: 0 .25em 0 .25em;
        margin: 1em 1em 1em 0;
        font-size: 12pt;
        color: #3e3e3e;
        border-radius: 3px;
      }
      pre.pre-box {
        background: #343a40;
        color: #fff;
        padding: 10px;
        border: 1px solid gray;
        display: inline-block;
        border-radius: 4px;
        width: 97%;
        font-size: 1.25em;
        overflow-x: auto;
        max-height: 60em;
        overflow-y: auto;
        max-width: 85%;
      }
      .summary {
        width: 100%;
        height: auto;
        padding: 0 0 .5em 0;
        border-radius: 6px;
        border: 1px solid #cccddd;
        background: white;
      }
      .summary-bullet {
        width: 100%;
        height: auto;
        display: flex;
        flex-wrap: wrap;
        padding: .5em .1em .1em .5em;
      }
      .bullet-left {
        width: 25%;
        font-weight: bold;
        font-size: 100%;
      }
      .bullet-right {
        width: auto;
        font-family: monospace;
        font-size: 110%;
      }
      .quick-summary {
        width: 70%;
        display: flex;
        margin: 0 auto 0 0;
        font-weight: bold;
        font-size: 1.2em;
      }
      .darkgreen {
        color: green;
      }
      .darkred {
        color: red;
        padding: 0 0 0 2em;
      }
      .darkgrey {
        color: grey;
        padding: 0 0 0 2em;
      }
      .meter {
        border: 1px solid black;
        margin: 0 .5em 0 auto;
        display: flex;
        height: 25px;
        width: 45%;
      }
      @media only screen and (max-width: 600px) {
        .meter {
          display: none;
        }
      }
      .meter-green {
        height: 100%;
        background: green;
        width: {{ .PercentPassed -}}%;
      }
      .meter-red {
        height: 100%;
        background: red;
        width: {{ .PercentFailed -}}%;
      }
      .meter-grey {
        height: 100%;
        background: grey;
        width: {{ .PercentSkipped -}}%;
      }
      .subcategory {
        background: white;
        padding: 0px 20px 20px 20px;
        border: 1px solid #cccddd;
        border-radius: 6px;
      }
      h2 {
        margin-top: 45px;
      }
      h4 {
        vertical-align: bottom;
        cursor: pointer;
      }
    </style>
    <script>
      function toggleOutput(id) {
        var elem = document.getElementById(id);
        var button = document.getElementById(id + "-button");
        if (elem.style['display'] === 'block') {
          button.innerHTML = "+";
          elem.style['display'] = 'none';
        } else {
          button.innerHTML = "-";
          elem.style['display'] = 'block';
        }
      }
    </script>
  </head>
  <body>
    <h1>OCI Distribution Conformance Tests</h1>
    <table>
      <tr>
      </tr>
      <tr>
        <td class="bullet-left">Summary</td>
        <td>
          <div class="quick-summary">
            {{- if gt .NumPassed 0 -}}
              <span class="darkgreen">
              {{- if .AllPassed -}}All {{ end -}}{{ .NumPassed }} passed</span>
            {{- end -}}
            {{- if gt .NumFailed 0 -}}
              <span class="darkred">
              {{- if .AllFailed -}}All {{ end -}}{{ .NumFailed }} failed</span>
            {{- end -}}
            {{- if gt .NumSkipped 0 -}}
              <span class="darkgrey">
              {{- if .AllSkipped -}}All {{ end -}}{{ .NumSkipped }} skipped</span>
            {{- end -}}
            <div class="meter">
              <div class="meter-green"></div>
              <div class="meter-red"></div>
              <div class="meter-grey"></div>
            </div>
          </div>
        </td>
      </tr>
      <tr>
        <td class="bullet-left">Start Time</td>
        <td>{{ .StartTimeString }}</td>
      </tr>
      <tr>
        <td class="bullet-left">End Time</td>
        <td>{{ .EndTimeString }}</td>
      </tr>
      <tr>
        <td class="bullet-left">Time Elapsed</td>
        <td>{{ .RunTime }}</td>
      </tr>
      <tr>
        <td class="bullet-left">Test Version</td>
        <td>{{ .Version }}</td>
      </tr>
      <tr>
        <td class="bullet-left">Configuration</td>
        <td><div class="bullet-right">
          {{ range $i, $s := .EnvironmentVariables }}
            {{ $s }}<br />
          {{ end }}
        </div></td>
      </tr>
    </table>

    <div>
      {{with .Suite}}
        {{$suite := .M}}
        {{range $i, $suiteKey := .Keys}}
          {{$wf := index $suite $suiteKey}}
          {{with $wf}}
            {{ if .IsEnabled }}
              <h2>{{$suiteKey}}</h2>
              <div class="subcategory">
              {{$workflow := .M}}
              {{range $j, $workflowKey := .Keys}}
                <h3>{{$workflowKey}}</h3>
                {{$ctg := index $workflow $workflowKey}}
                {{with $ctg}}
                  {{$category := .M}}
                  {{range $k, $categoryKey := .Keys}}
                    {{$s := index $category $categoryKey}}
                    {{if eq $s.State.String "failed"}}
                      <div class="result red">
                        <div id="output-box-{{$s.ID}}-button" class="toggle" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">+</div>
                        <h4 style="display: inline;" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">{{$s.Title}}</h4>
                        <br>
                        <div>
                          <div id="output-box-{{$s.ID}}" style="display: none;">
                            <pre class="pre-box">{{$s.CombinedOutput}}</pre>
                          </div>
                        </div>
                        <pre class="fail-message">{{$s.FailureMessage}}</pre>
                        <br>
                      </div>
                    {{else if eq $s.State.String "passed"}}
                      <div class="result green">
                        <div id="output-box-{{$s.ID}}-button" class="toggle" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">+</div>
                        <h4 style="display: inline;" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">{{$s.Title}}</h4>
                        <br>
                        <div id="output-box-{{$s.ID}}" style="display: none;">
                          <pre class="pre-box">{{$s.CombinedOutput}}</pre>
                        </div>
                      </div>
                    {{else if eq $s.State.String "skipped"}}
                      <div class="result grey">
                        <div id="output-box-{{$s.ID}}-button" class="toggle" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">+</div>
                        <h4 style="display: inline;" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">{{$s.Title}}</h4>
                        <br>
                        <div id="output-box-{{$s.ID}}" style="display: none;">
                          <pre class="pre-box">{{$s.FailureMessage}}</pre>
                        </div>
                      </div>
                    {{else}}
                      <div class="result grey">
                        <div id="output-box-{{$s.ID}}-button" class="toggle" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">+</div>
                        <h4 style="display: inline;" onclick="javascript:toggleOutput('output-box-{{$s.ID}}')">{{$s.Title}}</h4>
                        <br>
                        <div id="output-box-{{$s.ID}}" style="display: none;">
                          <p>Unhandled state: {{ $s.State.String }}</p>
                          <pre class="pre-box">{{$s.CombinedOutput}}</pre>
                          <pre class="pre-box">{{$s.FailureMessage}}</pre>
                        </div>
                      </div>
                    {{end}}
                  {{end}}<br>
                {{end}}
              {{end}}
            {{end}}
          {{end}}
        </div>
        {{end}}
      {{end}}
    </div>
  </body>
</html>
`
)

type (
	summaryMap struct {
		M    map[string]snapShotList
		Keys []string
		Size int
	}

	suite struct {
		M    map[string]*workflow
		Keys []string
		Size int
	}

	workflow struct {
		M         map[string]*category
		IsEnabled bool
		Keys      []string
	}

	category struct {
		M    map[string]specSnapshot
		Keys []string
	}

	specSnapshot struct {
		types.SpecReport
		ID       int
		Title    string
		Category string
		Suite    string
		IsSetup  bool
	}

	snapShotList []specSnapshot

	httpDebugWriter struct {
		CapturedOutput []string
		debug          bool
	}

	httpDebugLogger struct {
		l *log.Logger
		w io.Writer
	}

	HTMLReporter struct {
		htmlReportFilename   string
		Suite                suite
		SpecSummaryMap       summaryMap
		EnvironmentVariables []string
		Report               types.Report
		debugLogger          *httpDebugWriter
		debugIndex           int
		enabledMap           map[string]bool
		NumTotal             int
		NumPassed            int
		NumFailed            int
		NumSkipped           int
		PercentPassed        int
		PercentFailed        int
		PercentSkipped       int
		startTime            time.Time
		endTime              time.Time
		StartTimeString      string
		EndTimeString        string
		RunTime              string
		AllPassed            bool
		AllFailed            bool
		AllSkipped           bool
		Version              string
	}
)

func (sm *summaryMap) Add(key string, sum *specSnapshot) {
	sm.M[key] = append(sm.M[key], *sum)
	sm.Size++

	if !sm.containsKey(key) {
		sm.Keys = append(sm.Keys, key)
	}
}

func (sm *summaryMap) containsKey(key string) bool {
	var containsKey bool
	for _, k := range sm.Keys {
		if k == key {
			containsKey = true
			break
		}
	}
	return containsKey
}

func newHTTPDebugWriter(debug bool) *httpDebugWriter {
	return &httpDebugWriter{debug: debug}
}

func (writer *httpDebugWriter) Write(b []byte) (int, error) {
	s := string(b)
	writer.CapturedOutput = append(writer.CapturedOutput, s)
	if writer.debug {
		fmt.Println(s)
	}

	return len(b), nil
}

func newHTTPDebugLogger(f io.Writer) *httpDebugLogger {
	debugLogger := &httpDebugLogger{w: f, l: log.New(f, "", log.Ldate|log.Lmicroseconds)}
	return debugLogger
}

func (l *httpDebugLogger) Errorf(format string, v ...interface{}) {
	l.output("ERROR "+format, v...)
}

func (l *httpDebugLogger) Warnf(format string, v ...interface{}) {
	l.output("WARN "+format, v...)
}

func (l *httpDebugLogger) Debugf(format string, v ...interface{}) {
	l.output("DEBUG "+format, v...)
}

var (
	redactRegexp  = regexp.MustCompile(`(?i)("?\w*(authorization|token|state)\w*"?(:|=)\s*)(")?\s*((bearer|basic)? )?[^\s&"]*(")?`)
	redactReplace = "$1$4$5*****$7"
)

func (l *httpDebugLogger) output(format string, v ...interface{}) {
	if len(v) == 0 {
		l.l.Print(redactRegexp.ReplaceAllString(format, redactReplace))
		return
	}
	_, err := l.w.Write([]byte(redactRegexp.ReplaceAllString(fmt.Sprintf(format, v...), redactReplace)))
	if err != nil {
		l.Errorf(err.Error())
	}
}

func newHTMLReporter(htmlReportFilename string) (h *HTMLReporter) {
	enabledMap := map[string]bool{
		titlePull:              true,
		titlePush:              true,
		titleContentDiscovery:  true,
		titleContentManagement: true,
	}

	if os.Getenv(envVarHideSkippedWorkflows) == "1" {
		enabledMap = map[string]bool{
			titlePull:              !userDisabled(pull),
			titlePush:              !userDisabled(push),
			titleContentDiscovery:  !userDisabled(contentDiscovery),
			titleContentManagement: !userDisabled(contentManagement),
		}
	}

	varsToCheck := []string{
		envVarRootURL,
		envVarNamespace,
		envVarUsername,
		envVarPassword,
		envVarDebug,
		envVarPull,
		envVarPush,
		envVarContentDiscovery,
		envVarContentManagement,
		envVarPushEmptyLayer,
		envVarBlobDigest,
		envVarManifestDigest,
		envVarTagName,
		envVarTagList,
		envVarHideSkippedWorkflows,
		envVarAuthScope,
		envVarCrossmountNamespace,
	}
	envVars := []string{}
	for _, v := range varsToCheck {
		var replacement string
		if envVar := os.Getenv(v); envVar != "" {
			replacement = envVar
			if strings.Contains(v, "PASSWORD") || strings.Contains(v, "USERNAME") {
				replacement = "*****"
			}
		} else {
			continue
		}
		envVars = append(envVars,
			fmt.Sprintf("%s=%s", v, replacement))
	}

	return &HTMLReporter{
		htmlReportFilename: htmlReportFilename,
		debugLogger:        httpWriter,
		enabledMap:         enabledMap,
		SpecSummaryMap:     summaryMap{M: make(map[string]snapShotList)},
		Suite: suite{
			M:    make(map[string]*workflow),
			Keys: []string{},
		},
		EnvironmentVariables: envVars,
		startTime:            time.Now(),
		StartTimeString:      time.Now().Format("Jan 2 15:04:05.000 -0700 MST"),
		Version:              Version,
	}
}

func (reporter *HTMLReporter) afterReport(r types.SpecReport) {
	b := new(bytes.Buffer)
	for _, co := range httpWriter.CapturedOutput[reporter.debugIndex:] {
		fmt.Fprintf(b, "%s\n", co)
	}
	r.CapturedStdOutErr = b.String()
	reporter.debugIndex = len(reporter.debugLogger.CapturedOutput)

	ct := r.ContainerHierarchyTexts
	suiteName, categoryName, titleText := ct[suiteIndex], ct[categoryIndex], r.LeafNodeText
	suite := &reporter.Suite
	//make the map of categories
	if _, ok := suite.M[suiteName]; !ok {
		suite.M[suiteName] = &workflow{M: make(map[string]*category), Keys: []string{},
			IsEnabled: reporter.enabledMap[suiteName]}
		suite.Keys = append(suite.Keys, suiteName)
	}
	//make the map of snapshots
	if _, ok := suite.M[suiteName].M[categoryName]; !ok {
		suite.M[suiteName].M[categoryName] = &category{M: make(map[string]specSnapshot), Keys: []string{}}
		z := suite.M[suiteName]
		z.Keys = append(z.Keys, categoryName)
	}
	z := suite.M[suiteName].M[categoryName]
	z.Keys = append(z.Keys, titleText)

	suite.M[suiteName].M[categoryName].M[titleText] = specSnapshot{
		SpecReport: r,
		Suite:      suiteName,
		Category:   categoryName,
		Title:      titleText,
		ID:         suite.Size,
		IsSetup:    (categoryName == setupString),
	}
	suite.Size++
}

func (reporter *HTMLReporter) endSuite(report types.Report) error {
	if reporter.htmlReportFilename == "" {
		// Reporting is disabled.
		return nil
	}
	reporter.Report = report
	reporter.endTime = time.Now()
	reporter.EndTimeString = reporter.endTime.Format("Jan 2 15:04:05.000 -0700 MST")
	reporter.RunTime = reporter.endTime.Sub(reporter.startTime).String()
	reporter.NumTotal = len(report.SpecReports)
	reporter.NumPassed = report.SpecReports.CountWithState(types.SpecStatePassed)
	reporter.NumSkipped = report.SpecReports.CountWithState(types.SpecStateSkipped)
	reporter.NumFailed = report.SpecReports.CountWithState(types.SpecStateFailed)
	reporter.PercentPassed = getPercent(reporter.NumPassed, reporter.NumTotal)
	reporter.PercentSkipped = getPercent(reporter.NumSkipped, reporter.NumTotal)
	reporter.PercentFailed = getPercent(reporter.NumFailed, reporter.NumTotal)
	reporter.AllPassed = reporter.NumPassed == reporter.NumTotal
	reporter.AllSkipped = reporter.NumSkipped == reporter.NumTotal
	reporter.AllFailed = reporter.NumFailed == reporter.NumTotal

	t, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("cannot parse report template: %v", err)
	}

	htmlReportFilenameAbsPath, err := filepath.Abs(reporter.htmlReportFilename)
	if err != nil {
		return err
	}

	htmlReportFile, err := os.Create(htmlReportFilenameAbsPath)
	if err != nil {
		return err
	}
	defer htmlReportFile.Close()

	err = t.ExecuteTemplate(htmlReportFile, "report", &reporter)
	if err != nil {
		return err
	}

	fmt.Printf("\nHTML report was created: %s", htmlReportFilenameAbsPath)
	return nil
}

func getPercent(i, of int) int {
	return int(math.Round(float64(i) / float64(of) * 100))
}
