package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Mock HTML response simulating autopkgtest results page
const mockHTMLWithErrors = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results for ovn</title></head>
<body>
<h2>ovn</h2>
<table class="table" style="width: auto">
  <tr>
    <th></th>
    <th>focal</th><th>jammy</th><th>noble</th>
  </tr>
  <tr>
    <th>amd64</th>
    <td class="pass">
      <a href="ovn/focal/amd64">pass</a>
    </td>
    <td class="pass">
      <a href="ovn/jammy/amd64">pass</a>
    </td>
    <td class="fail">
      <a href="ovn/noble/amd64">fail</a>
    </td>
  </tr>
  <tr>
    <th>arm64</th>
    <td class="pass">
      <a href="ovn/focal/arm64">pass</a>
    </td>
    <td class="regression">
      <a href="ovn/jammy/arm64">regression</a>
    </td>
    <td class="pass">
      <a href="ovn/noble/arm64">pass</a>
    </td>
  </tr>
</table>
</body>
</html>
`

const mockHTMLWithoutErrors = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results for test-pkg</title></head>
<body>
<h2>test-pkg</h2>
<table class="table">
  <tr>
    <th></th>
    <th>focal</th><th>jammy</th>
  </tr>
  <tr>
    <th>amd64</th>
    <td class="pass"><a href="test-pkg/focal/amd64">pass</a></td>
    <td class="pass"><a href="test-pkg/jammy/amd64">pass</a></td>
  </tr>
  <tr>
    <th>arm64</th>
    <td class="pass"><a href="test-pkg/focal/arm64">pass</a></td>
    <td class="pass"><a href="test-pkg/jammy/arm64">pass</a></td>
  </tr>
</table>
</body>
</html>
`

const mockHTMLWithResolute = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results for openvswitch</title></head>
<body>
<h2>openvswitch</h2>
<table class="table" style="width: auto">
  <tr>
    <th></th>
    <th>focal</th><th>jammy</th><th>noble</th><th>questing</th><th>resolute</th>
  </tr>
  <tr>
    <th>amd64</th>
    <td class="pass"><a href="openvswitch/focal/amd64">pass</a></td>
    <td class="pass"><a href="openvswitch/jammy/amd64">pass</a></td>
    <td class="tmpfail"><a href="openvswitch/noble/amd64">tmpfail</a></td>
    <td class="fail"><a href="openvswitch/questing/amd64">fail</a></td>
    <td class="pass"><a href="openvswitch/resolute/amd64">pass</a></td>
  </tr>
  <tr>
    <th>arm64</th>
    <td class="pass"><a href="openvswitch/focal/arm64">pass</a></td>
    <td class="pass"><a href="openvswitch/jammy/arm64">pass</a></td>
    <td class="tmpfail"><a href="openvswitch/noble/arm64">tmpfail</a></td>
    <td class="tmpfail"><a href="openvswitch/questing/arm64">tmpfail</a></td>
    <td class="pass"><a href="openvswitch/resolute/arm64">pass</a></td>
  </tr>
</table>
</body>
</html>
`

const mockHTMLWithTbody = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results for pkg-tbody</title></head>
<body>
<h2>pkg-tbody</h2>
<table class="table">
  <tbody>
  <tr>
    <th></th>
    <th>jammy</th><th>noble</th>
  </tr>
  <tr>
    <th>amd64</th>
    <td class="pass"><a href="pkg-tbody/jammy/amd64">pass</a></td>
    <td class="fail"><a href="pkg-tbody/noble/amd64">fail</a></td>
  </tr>
  </tbody>
</table>
</body>
</html>
`

const mockHTMLEmpty = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results</title></head>
<body>
<h1>No results found</h1>
</body>
</html>
`

func TestNewScraper(t *testing.T) {
	s := NewScraper()

	if s == nil {
		t.Fatal("NewScraper returned nil")
	}

	if s.BaseURL != "https://autopkgtest.ubuntu.com" {
		t.Errorf("Expected BaseURL to be https://autopkgtest.ubuntu.com, got %s", s.BaseURL)
	}

	if s.Client == nil {
		t.Error("Expected Client to be initialized")
	}
}

func TestParseHTMLWithErrors(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if results.Package != "ovn" {
		t.Errorf("Expected package name 'ovn', got '%s'", results.Package)
	}

	// 2 architectures × 3 releases = 6 tests
	if len(results.Tests) != 6 {
		t.Errorf("Expected 6 test results, got %d", len(results.Tests))
	}

	// Should have exactly 2 errors: noble/amd64 (fail) and jammy/arm64 (regression)
	if len(results.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(results.Errors))
	}

	// Verify specific release assignments
	releaseArchStatus := map[string]string{}
	for _, test := range results.Tests {
		key := test.Release + "/" + test.Architecture
		releaseArchStatus[key] = test.Status
	}

	expected := map[string]string{
		"focal/amd64": "pass",
		"jammy/amd64": "pass",
		"noble/amd64": "fail",
		"focal/arm64": "pass",
		"jammy/arm64": "regression",
		"noble/arm64": "pass",
	}
	for key, wantStatus := range expected {
		got, ok := releaseArchStatus[key]
		if !ok {
			t.Errorf("Missing result for %s", key)
		} else if !strings.EqualFold(got, wantStatus) {
			t.Errorf("For %s: expected status %q, got %q", key, wantStatus, got)
		}
	}
}

func TestParseHTMLWithoutErrors(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithoutErrors, "test-pkg", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if results.Package != "test-pkg" {
		t.Errorf("Expected package name 'test-pkg', got '%s'", results.Package)
	}

	// 2 archs × 2 releases = 4
	if len(results.Tests) != 4 {
		t.Errorf("Expected 4 tests, got %d", len(results.Tests))
	}

	if len(results.Errors) != 0 {
		t.Errorf("Expected 0 errors for all-passing tests, got %d", len(results.Errors))
	}
}

func TestParseHTMLEmpty(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLEmpty, "empty-pkg", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if results.Package != "empty-pkg" {
		t.Errorf("Expected package name 'empty-pkg', got '%s'", results.Package)
	}

	if len(results.Tests) != 0 {
		t.Errorf("Expected 0 tests for empty HTML, got %d", len(results.Tests))
	}
}

func TestParseHTMLWithResolute(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithResolute, "openvswitch", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	// 2 archs × 5 releases = 10
	if len(results.Tests) != 10 {
		t.Errorf("Expected 10 test results, got %d", len(results.Tests))
	}

	// Verify resolute results are present and correctly assigned
	var resoluteResults []TestResult
	for _, test := range results.Tests {
		if test.Release == "resolute" {
			resoluteResults = append(resoluteResults, test)
		}
	}
	if len(resoluteResults) != 2 {
		t.Fatalf("Expected 2 resolute results, got %d", len(resoluteResults))
	}
	for _, r := range resoluteResults {
		if r.Status != "pass" {
			t.Errorf("Expected resolute/%s to be pass, got %s", r.Architecture, r.Status)
		}
	}

	// Verify the last release (resolute) has correct log URLs
	for _, r := range resoluteResults {
		expectedSuffix := "openvswitch/resolute/" + r.Architecture
		if !strings.HasSuffix(r.LogURL, expectedSuffix) {
			t.Errorf("Expected LogURL to end with %q, got %q", expectedSuffix, r.LogURL)
		}
	}
}

func TestParseHTMLWithResoluteFilter(t *testing.T) {
	s := NewScraper()
	filter := &Filter{Release: "resolute"}
	results, err := s.ParseHTML(mockHTMLWithResolute, "openvswitch", filter)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if len(results.Tests) != 2 {
		t.Errorf("Expected 2 results for resolute filter, got %d", len(results.Tests))
	}

	for _, test := range results.Tests {
		if !strings.EqualFold(test.Release, "resolute") {
			t.Errorf("Expected only resolute release, got %s", test.Release)
		}
	}
}

func TestParseHTMLWithTbody(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithTbody, "pkg-tbody", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	if len(results.Tests) != 2 {
		t.Errorf("Expected 2 tests, got %d", len(results.Tests))
	}

	// Verify correct release assignment
	for _, test := range results.Tests {
		if test.Architecture != "amd64" {
			t.Errorf("Expected architecture amd64, got %s", test.Architecture)
		}
	}
	releaseStatus := map[string]string{}
	for _, test := range results.Tests {
		releaseStatus[test.Release] = test.Status
	}
	if releaseStatus["jammy"] != "pass" {
		t.Errorf("Expected jammy/amd64 = pass, got %s", releaseStatus["jammy"])
	}
	if releaseStatus["noble"] != "fail" {
		t.Errorf("Expected noble/amd64 = fail, got %s", releaseStatus["noble"])
	}
}

func TestFetchPackageResultsWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/packages/ovn" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHTMLWithErrors))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := NewScraper()
	s.BaseURL = server.URL

	results, err := s.FetchPackageResults("ovn")
	if err != nil {
		t.Fatalf("FetchPackageResults failed: %v", err)
	}

	if results.Package != "ovn" {
		t.Errorf("Expected package name 'ovn', got '%s'", results.Package)
	}

	if len(results.Tests) != 6 {
		t.Errorf("Expected 6 test results, got %d", len(results.Tests))
	}
}

func TestFetchPackageResults404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	s := NewScraper()
	s.BaseURL = server.URL

	_, err := s.FetchPackageResults("nonexistent")
	if err == nil {
		t.Error("Expected error for 404 response, got nil")
	}
}

func TestReportErrors(t *testing.T) {
	results := &PackageResults{
		Package: "test-pkg",
		Errors: []TestResult{
			{
				Status:       "FAIL",
				Release:      "noble",
				Architecture: "amd64",
				Duration:     "15m",
			},
			{
				Status:       "REGRESSION",
				Release:      "jammy",
				Architecture: "arm64",
			},
		},
	}

	report := results.ReportErrors()

	if report == "" {
		t.Error("Expected non-empty report")
	}

	expectedStrings := []string{
		"test-pkg",
		"FAIL",
		"REGRESSION",
		"amd64",
		"arm64",
		"noble",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(report, expected) {
			t.Errorf("Expected report to contain '%s'", expected)
		}
	}
}

func TestReportErrorsEmpty(t *testing.T) {
	results := &PackageResults{
		Package: "test-pkg",
		Errors:  []TestResult{},
	}

	report := results.ReportErrors()

	if !strings.Contains(report, "No errors found") {
		t.Error("Expected 'No errors found' message for empty errors")
	}
}

func TestFilterByRelease(t *testing.T) {
	s := NewScraper()
	filter := &Filter{Release: "noble"}
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", filter)

	if err != nil {
		t.Fatalf("ParseHTML with filter failed: %v", err)
	}

	// Should have exactly 2 noble results (amd64 + arm64)
	if len(results.Tests) != 2 {
		t.Errorf("Expected 2 results for noble, got %d", len(results.Tests))
	}

	for _, test := range results.Tests {
		if !strings.EqualFold(test.Release, "noble") {
			t.Errorf("Expected only noble release, got %s", test.Release)
		}
	}
}

func TestFilterByArchitecture(t *testing.T) {
	s := NewScraper()
	filter := &Filter{Architecture: "amd64"}
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", filter)

	if err != nil {
		t.Fatalf("ParseHTML with filter failed: %v", err)
	}

	// Should have exactly 3 amd64 results (focal + jammy + noble)
	if len(results.Tests) != 3 {
		t.Errorf("Expected 3 results for amd64, got %d", len(results.Tests))
	}

	for _, test := range results.Tests {
		if !strings.EqualFold(test.Architecture, "amd64") {
			t.Errorf("Expected only amd64 architecture, got %s", test.Architecture)
		}
	}
}

func TestFilterByReleaseAndArchitecture(t *testing.T) {
	s := NewScraper()
	filter := &Filter{
		Release:      "jammy",
		Architecture: "arm64",
	}
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", filter)

	if err != nil {
		t.Fatalf("ParseHTML with filter failed: %v", err)
	}

	if len(results.Tests) != 1 {
		t.Fatalf("Expected 1 result for jammy/arm64, got %d", len(results.Tests))
	}

	test := results.Tests[0]
	if !strings.EqualFold(test.Release, "jammy") {
		t.Errorf("Expected jammy release, got %s", test.Release)
	}
	if !strings.EqualFold(test.Architecture, "arm64") {
		t.Errorf("Expected arm64 architecture, got %s", test.Architecture)
	}
	if !strings.EqualFold(test.Status, "regression") {
		t.Errorf("Expected regression status for jammy/arm64, got %s", test.Status)
	}
}

func TestFetchPackageResultsFiltered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/packages/ovn" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHTMLWithErrors))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	s := NewScraper()
	s.BaseURL = server.URL

	filter := &Filter{Release: "noble"}
	results, err := s.FetchPackageResultsFiltered("ovn", filter)
	if err != nil {
		t.Fatalf("FetchPackageResultsFiltered failed: %v", err)
	}

	if len(results.Tests) != 2 {
		t.Errorf("Expected 2 results for noble, got %d", len(results.Tests))
	}

	for _, test := range results.Tests {
		if !strings.EqualFold(test.Release, "noble") {
			t.Errorf("Expected only noble release, got %s", test.Release)
		}
	}
}

func TestLogURLsAreAbsolute(t *testing.T) {
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	for _, test := range results.Tests {
		if test.LogURL != "" && !strings.HasPrefix(test.LogURL, "https://") {
			t.Errorf("LogURL should be absolute, got %s", test.LogURL)
		}
	}
}

func TestReleaseAlignmentAcrossAllColumns(t *testing.T) {
	// Regression test: verify that each test result has the correct release
	// assigned, particularly for the last column (which was previously
	// dropped due to an off-by-one error).
	s := NewScraper()
	results, err := s.ParseHTML(mockHTMLWithResolute, "openvswitch", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	// Build map of LogURL suffix → release for verification.
	// The href in the HTML encodes the correct release, so we can cross-check.
	for _, test := range results.Tests {
		// LogURL looks like "https://autopkgtest.ubuntu.com/openvswitch/<release>/<arch>"
		parts := strings.Split(test.LogURL, "/")
		if len(parts) < 3 {
			t.Errorf("Unexpected LogURL format: %s", test.LogURL)
			continue
		}
		urlRelease := parts[len(parts)-2]
		if !strings.EqualFold(urlRelease, test.Release) {
			t.Errorf("Release mismatch: TestResult.Release=%q but LogURL contains release=%q (URL: %s)",
				test.Release, urlRelease, test.LogURL)
		}
	}
}
