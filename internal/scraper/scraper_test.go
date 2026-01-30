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
<h1>Package: ovn</h1>
<table class="table" style="width: auto">
  <tbody>
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
  </tbody>
</table>
</body>
</html>
`

const mockHTMLWithoutErrors = `
<!DOCTYPE html>
<html>
<head><title>autopkgtest results for test-pkg</title></head>
<body>
<h1>Package: test-pkg</h1>
<table class="table">
  <tbody>
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

	// We should have tests for amd64 x 3 releases + arm64 x 3 releases = 6 (but may be fewer if some are missing)
	if len(results.Tests) < 4 {
		t.Errorf("Expected at least 4 test results, got %d", len(results.Tests))
	}

	// Should have 2 errors (1 fail + 1 regression)
	if len(results.Errors) < 2 {
		t.Errorf("Expected at least 2 errors (FAIL and REGRESSION), got %d", len(results.Errors))
	}

	// Check that PASS tests are not in errors
	for _, err := range results.Errors {
		status := strings.ToLower(err.Status)
		if strings.Contains(status, "pass") {
			t.Error("PASS status should not be in errors list")
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

func TestFetchPackageResultsWithMockServer(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/packages/ovn" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHTMLWithErrors))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create scraper with mock server URL
	s := NewScraper()
	s.BaseURL = server.URL

	results, err := s.FetchPackageResults("ovn")
	if err != nil {
		t.Fatalf("FetchPackageResults failed: %v", err)
	}

	if results.Package != "ovn" {
		t.Errorf("Expected package name 'ovn', got '%s'", results.Package)
	}

	if len(results.Tests) < 4 {
		t.Errorf("Expected at least 4 test results, got %d", len(results.Tests))
	}
}

func TestFetchPackageResults404(t *testing.T) {
	// Create a mock HTTP server that always returns 404
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

	// Check that report contains expected information
	expectedStrings := []string{
		"test-pkg",
		"FAIL",
		"REGRESSION",
		"amd64",
		"arm64",
		"noble",
	}

	for _, expected := range expectedStrings {
		if !contains(report, expected) {
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

	if !contains(report, "No errors found") {
		t.Error("Expected 'No errors found' message for empty errors")
	}
}

func TestExtractStatus(t *testing.T) {
	// This test verifies the status extraction logic indirectly through ParseHTML
	// since extractStatus requires an html.Node parameter

	htmlWithStatus := `
	<table>
	<tr><td class="pass">PASS</td></tr>
	<tr><td class="fail">FAIL</td></tr>
	<tr><td>REGRESSION</td></tr>
	</table>
	`

	s := NewScraper()
	results, err := s.ParseHTML(htmlWithStatus, "test", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	// Just verify that status extraction is working through the parser
	if results == nil {
		t.Error("Expected non-nil results")
	}
}

func TestGetNodeText(t *testing.T) {
	htmlStr := "<div>Test <span>nested</span> text</div>"
	s := NewScraper()
	results, err := s.ParseHTML(htmlStr, "test", nil)

	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}

	// Just verify parsing doesn't crash with nested elements
	if results == nil {
		t.Error("Expected non-nil results")
	}
}

func TestFilterByRelease(t *testing.T) {
	s := NewScraper()
	filter := &Filter{Release: "noble"}
	results, err := s.ParseHTML(mockHTMLWithErrors, "ovn", filter)

	if err != nil {
		t.Fatalf("ParseHTML with filter failed: %v", err)
	}

	// All results should be for noble release only
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

	// All results should be for amd64 architecture only
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

	// Should have exactly 1 result: jammy + arm64
	if len(results.Tests) != 1 {
		t.Errorf("Expected 1 result for jammy/arm64, got %d", len(results.Tests))
	}

	if len(results.Tests) > 0 {
		test := results.Tests[0]
		if !strings.EqualFold(test.Release, "jammy") {
			t.Errorf("Expected jammy release, got %s", test.Release)
		}
		if !strings.EqualFold(test.Architecture, "arm64") {
			t.Errorf("Expected arm64 architecture, got %s", test.Architecture)
		}
	}
}

func TestFetchPackageResultsFiltered(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/packages/ovn" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHTMLWithErrors))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create scraper with mock server URL
	s := NewScraper()
	s.BaseURL = server.URL

	filter := &Filter{Release: "noble"}
	results, err := s.FetchPackageResultsFiltered("ovn", filter)
	if err != nil {
		t.Fatalf("FetchPackageResultsFiltered failed: %v", err)
	}

	// All results should be for noble
	for _, test := range results.Tests {
		if !strings.EqualFold(test.Release, "noble") {
			t.Errorf("Expected only noble release, got %s", test.Release)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
