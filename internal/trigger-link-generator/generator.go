package triggerlinkgenerator

import (
	"fmt"
	"net/url"
	"strings"
)

// LinkRequest represents a request to generate an autopkgtest trigger link
type LinkRequest struct {
	Package       string   // Source package name (required)
	Version       string   // Package version (optional, used in trigger param)
	Triggers      []string // Custom trigger list (optional, overrides package/version; multiple triggers supported)
	Architectures []string // List of architectures to test (optional)
	Suite         string   // Ubuntu release codename (required, e.g., "noble", "mantic")
	PPA           string   // PPA name for testing (optional, format: "user/ppa-name")
	AllProposed   bool     // Install all packages from proposed pocket (optional)
}

// LinkResponse represents the result of generating trigger URLs
type LinkResponse struct {
	URLs    []string // Generated trigger URLs (one per architecture, or one if no arch specified)
	Message string   // Human-readable message
}

// Generator handles generating autopkgtest trigger URLs
type Generator struct {
	BaseURL string
}

// NewGenerator creates a new generator instance
func NewGenerator() *Generator {
	return &Generator{
		BaseURL: "https://autopkgtest.ubuntu.com/request.cgi",
	}
}

// GenerateLinks creates autopkgtest trigger URLs based on the request
// These URLs must be opened by a user authenticated to Launchpad with
// upload rights for the package.
func (g *Generator) GenerateLinks(req *LinkRequest) (*LinkResponse, error) {
	// Validate required fields
	if req.Package == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if req.Suite == "" {
		return nil, fmt.Errorf("suite (release) is required")
	}

	// Determine trigger parameter
	var trigger string
	if len(req.Triggers) > 0 {
		// Join multiple triggers with spaces for the URL
		trigger = strings.Join(req.Triggers, " ")
	} else if req.Version != "" {
		trigger = fmt.Sprintf("%s/%s", req.Package, req.Version)
	} else {
		// Use migration-reference/0 as a safe default
		trigger = "migration-reference/0"
	}

	var urls []string
	var message string

	// If architectures are specified, generate one URL per arch
	if len(req.Architectures) > 0 {
		for _, arch := range req.Architectures {
			generatedURL := g.buildURL(req.Package, req.Suite, arch, trigger, req.PPA, req.AllProposed)
			urls = append(urls, generatedURL)
		}
		message = fmt.Sprintf("Generated %d trigger URL(s) for package '%s' on %s (%s)",
			len(urls), req.Package, req.Suite, strings.Join(req.Architectures, ", "))
	} else {
		// Generate a single URL without architecture specification
		generatedURL := g.buildURL(req.Package, req.Suite, "", trigger, req.PPA, req.AllProposed)
		urls = append(urls, generatedURL)
		message = fmt.Sprintf("Generated trigger URL for package '%s' on %s (all architectures)",
			req.Package, req.Suite)
	}

	return &LinkResponse{
		URLs:    urls,
		Message: message,
	}, nil
}

// buildURL constructs a single autopkgtest trigger URL
func (g *Generator) buildURL(pkg, suite, arch, trigger, ppa string, allProposed bool) string {
	params := url.Values{}
	params.Add("release", suite)
	params.Add("package", pkg)
	params.Add("trigger", trigger)

	if arch != "" {
		params.Add("arch", arch)
	}

	if ppa != "" {
		params.Add("ppa", ppa)
	}

	if allProposed {
		params.Add("all-proposed", "1")
	}

	return fmt.Sprintf("%s?%s", g.BaseURL, params.Encode())
}

// String creates a formatted string representation of a link request
func (req *LinkRequest) String() string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Package:\t%s\n", req.Package))
	result.WriteString(fmt.Sprintf("Suite:\t%s\n", req.Suite))

	if req.Version != "" {
		result.WriteString(fmt.Sprintf("Version:\t%s\n", req.Version))
	}
	if len(req.Triggers) > 0 {
		result.WriteString(fmt.Sprintf("Trigger(s):\t%s\n", strings.Join(req.Triggers, ", ")))
	}
	if len(req.Architectures) > 0 {
		result.WriteString(fmt.Sprintf("Arch(s):\t%s\n", strings.Join(req.Architectures, ", ")))
	} else {
		result.WriteString("Arch(s):\tall\n")
	}
	if req.PPA != "" {
		result.WriteString(fmt.Sprintf("PPA:\t%s\n", req.PPA))
	}
	if req.AllProposed {
		result.WriteString("All-Proposed:\tyes\n")
	}

	return result.String()
}

// String creates a formatted string representation of the link response
func (resp *LinkResponse) String() string {
	var result strings.Builder
	result.WriteString(resp.Message + "\n\n")

	if len(resp.URLs) == 1 {
		result.WriteString("Trigger URL:\n")
		result.WriteString(resp.URLs[0] + "\n")
	} else {
		result.WriteString("Trigger URLs:\n")
		for i, url := range resp.URLs {
			result.WriteString(fmt.Sprintf("%d. %s\n", i+1, url))
		}
	}

	return result.String()
}
