package triggerlinkgenerator

import (
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator()

	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}

	if gen.BaseURL != "https://autopkgtest.ubuntu.com/request.cgi" {
		t.Errorf("Expected BaseURL to be https://autopkgtest.ubuntu.com/request.cgi, got %s", gen.BaseURL)
	}
}

func TestGenerateLinksBasic(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package: "testpkg",
		Suite:   "noble",
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	if len(resp.URLs) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(resp.URLs))
	}

	url := resp.URLs[0]
	if !strings.Contains(url, "package=testpkg") {
		t.Errorf("URL should contain package=testpkg, got %s", url)
	}
	if !strings.Contains(url, "release=noble") {
		t.Errorf("URL should contain release=noble, got %s", url)
	}
	if !strings.Contains(url, "trigger=migration-reference%2F0") {
		t.Errorf("URL should contain default trigger, got %s", url)
	}
}

func TestGenerateLinksWithVersion(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package: "testpkg",
		Suite:   "jammy",
		Version: "1.2.3-1",
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	url := resp.URLs[0]
	if !strings.Contains(url, "trigger=testpkg%2F1.2.3-1") {
		t.Errorf("URL should contain trigger with version, got %s", url)
	}
}

func TestGenerateLinksWithArchitectures(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package:       "testpkg",
		Suite:         "noble",
		Architectures: []string{"amd64", "arm64"},
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	if len(resp.URLs) != 2 {
		t.Errorf("Expected 2 URLs (one per arch), got %d", len(resp.URLs))
	}

	// Check first URL has amd64
	if !strings.Contains(resp.URLs[0], "arch=amd64") {
		t.Errorf("First URL should contain arch=amd64, got %s", resp.URLs[0])
	}

	// Check second URL has arm64
	if !strings.Contains(resp.URLs[1], "arch=arm64") {
		t.Errorf("Second URL should contain arch=arm64, got %s", resp.URLs[1])
	}
}

func TestGenerateLinksWithPPA(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package: "testpkg",
		Suite:   "jammy",
		PPA:     "user/test-ppa",
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	url := resp.URLs[0]
	if !strings.Contains(url, "ppa=user%2Ftest-ppa") {
		t.Errorf("URL should contain encoded PPA, got %s", url)
	}
}

func TestGenerateLinksWithAllProposed(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package:     "testpkg",
		Suite:       "noble",
		AllProposed: true,
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	url := resp.URLs[0]
	if !strings.Contains(url, "all-proposed=1") {
		t.Errorf("URL should contain all-proposed=1, got %s", url)
	}
}

func TestGenerateLinksWithCustomTrigger(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package:  "testpkg",
		Suite:    "noble",
		Version:  "1.0.0",                    // Should be ignored
		Triggers: []string{"custom-trigger"}, // Should override version
	}

	resp, err := gen.GenerateLinks(req)
	if err != nil {
		t.Fatalf("GenerateLinks failed: %v", err)
	}

	url := resp.URLs[0]
	if !strings.Contains(url, "trigger=custom-trigger") {
		t.Errorf("URL should contain custom trigger, got %s", url)
	}
	if strings.Contains(url, "1.0.0") {
		t.Errorf("URL should not contain version when custom trigger is set, got %s", url)
	}
}

func TestGenerateLinksMissingPackage(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Suite: "noble",
	}

	_, err := gen.GenerateLinks(req)
	if err == nil {
		t.Error("Expected error for missing package, got nil")
	}
	if !strings.Contains(err.Error(), "package name is required") {
		t.Errorf("Expected 'package name is required' error, got: %v", err)
	}
}

func TestGenerateLinksMissingSuite(t *testing.T) {
	gen := NewGenerator()
	req := &LinkRequest{
		Package: "testpkg",
	}

	_, err := gen.GenerateLinks(req)
	if err == nil {
		t.Error("Expected error for missing suite, got nil")
	}
	if !strings.Contains(err.Error(), "suite (release) is required") {
		t.Errorf("Expected 'suite (release) is required' error, got: %v", err)
	}
}

func TestLinkRequestString(t *testing.T) {
	req := &LinkRequest{
		Package:       "testpkg",
		Suite:         "noble",
		Version:       "1.2.3",
		Architectures: []string{"amd64", "arm64"},
		PPA:           "user/ppa",
		AllProposed:   true,
	}

	str := req.String()

	// Check that all fields are represented
	if !strings.Contains(str, "testpkg") {
		t.Error("String should contain package name")
	}
	if !strings.Contains(str, "noble") {
		t.Error("String should contain suite")
	}
	if !strings.Contains(str, "1.2.3") {
		t.Error("String should contain version")
	}
	if !strings.Contains(str, "amd64, arm64") {
		t.Error("String should contain architectures")
	}
	if !strings.Contains(str, "user/ppa") {
		t.Error("String should contain PPA")
	}
	if !strings.Contains(str, "yes") {
		t.Error("String should indicate all-proposed is enabled")
	}
}

func TestLinkResponseString(t *testing.T) {
	resp := &LinkResponse{
		URLs:    []string{"http://example.com/test1", "http://example.com/test2"},
		Message: "Test message",
	}

	str := resp.String()

	if !strings.Contains(str, "Test message") {
		t.Error("String should contain message")
	}
	if !strings.Contains(str, "http://example.com/test1") {
		t.Error("String should contain first URL")
	}
	if !strings.Contains(str, "http://example.com/test2") {
		t.Error("String should contain second URL")
	}
}

func TestBuildURL(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name        string
		pkg         string
		suite       string
		arch        string
		trigger     string
		ppa         string
		allProposed bool
		wantSubstr  []string
	}{
		{
			name:        "basic",
			pkg:         "pkg",
			suite:       "noble",
			arch:        "",
			trigger:     "pkg/1.0",
			ppa:         "",
			allProposed: false,
			wantSubstr:  []string{"package=pkg", "release=noble", "trigger=pkg%2F1.0"},
		},
		{
			name:        "with arch",
			pkg:         "pkg",
			suite:       "noble",
			arch:        "amd64",
			trigger:     "pkg/1.0",
			ppa:         "",
			allProposed: false,
			wantSubstr:  []string{"arch=amd64"},
		},
		{
			name:        "with ppa",
			pkg:         "pkg",
			suite:       "noble",
			arch:        "",
			trigger:     "pkg/1.0",
			ppa:         "user/ppa",
			allProposed: false,
			wantSubstr:  []string{"ppa=user%2Fppa"},
		},
		{
			name:        "with all-proposed",
			pkg:         "pkg",
			suite:       "noble",
			arch:        "",
			trigger:     "pkg/1.0",
			ppa:         "",
			allProposed: true,
			wantSubstr:  []string{"all-proposed=1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := gen.buildURL(tt.pkg, tt.suite, tt.arch, tt.trigger, tt.ppa, tt.allProposed)

			for _, substr := range tt.wantSubstr {
				if !strings.Contains(url, substr) {
					t.Errorf("URL should contain '%s', got: %s", substr, url)
				}
			}
		})
	}
}
