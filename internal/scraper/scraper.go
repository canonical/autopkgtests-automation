package scraper

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// TestResult represents a single autopkgtest result
type TestResult struct {
	Package      string
	Release      string // Ubuntu release (focal, jammy, noble, etc.)
	Architecture string
	Status       string
	Duration     string
	Trigger      string
	LogURL       string
}

// PackageResults contains all test results for a package
type PackageResults struct {
	Package string
	Tests   []TestResult
	Errors  []TestResult
}

// Filter represents filter criteria for test results
type Filter struct {
	Release      string // Filter by specific release (e.g., "noble", "jammy")
	Architecture string // Filter by specific architecture (e.g., "amd64", "arm64")
}

// Scraper handles fetching and parsing autopkgtest results
type Scraper struct {
	BaseURL string
	Client  *http.Client
}

// NewScraper creates a new scraper instance
func NewScraper() *Scraper {
	return &Scraper{
		BaseURL: "https://autopkgtest.ubuntu.com",
		Client:  &http.Client{},
	}
}

// FetchPackageResults fetches and parses autopkgtest results for a package
func (s *Scraper) FetchPackageResults(packageName string) (*PackageResults, error) {
	return s.FetchPackageResultsFiltered(packageName, nil)
}

// FetchPackageResultsFiltered fetches and parses autopkgtest results for a package with optional filtering
func (s *Scraper) FetchPackageResultsFiltered(packageName string, filter *Filter) (*PackageResults, error) {
	url := fmt.Sprintf("%s/packages/%s", s.BaseURL, packageName)

	resp, err := s.Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package results: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return s.ParseHTML(string(body), packageName, filter)
}

// ParseHTML parses the HTML content and extracts test results
func (s *Scraper) ParseHTML(htmlContent string, packageName string, filter *Filter) (*PackageResults, error) {
	results := &PackageResults{
		Package: packageName,
		Tests:   []TestResult{},
		Errors:  []TestResult{},
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Parse the HTML to extract test results from the matrix table
	s.parseMatrixTable(doc, results)

	// Apply filters if provided
	if filter != nil {
		results.Tests = s.applyFilter(results.Tests, filter)
	}

	// Filter errors (tests with non-passing status)
	for _, test := range results.Tests {
		if !isPassingStatus(test.Status) {
			results.Errors = append(results.Errors, test)
		}
	}

	return results, nil
}

// applyFilter filters test results based on the provided criteria
func (s *Scraper) applyFilter(tests []TestResult, filter *Filter) []TestResult {
	if filter == nil {
		return tests
	}

	var filtered []TestResult
	for _, test := range tests {
		// Filter by release if specified
		if filter.Release != "" && !strings.EqualFold(test.Release, filter.Release) {
			continue
		}

		// Filter by architecture if specified
		if filter.Architecture != "" && !strings.EqualFold(test.Architecture, filter.Architecture) {
			continue
		}

		filtered = append(filtered, test)
	}

	return filtered
}

// isPassingStatus checks if a status indicates a passing test
func isPassingStatus(status string) bool {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	return normalizedStatus == "pass" || normalizedStatus == "✔ pass" ||
		normalizedStatus == "neutral" || normalizedStatus == "😐 neutral" ||
		strings.Contains(normalizedStatus, "pass")
}

// parseMatrixTable parses the main results matrix table
// The table has releases as columns (focal, jammy, noble, etc.) and architectures as rows
func (s *Scraper) parseMatrixTable(n *html.Node, results *PackageResults) {
	table := s.findMatrixTable(n)
	if table == nil {
		return
	}

	var releases []string
	var rows []*html.Node
	var foundHeader bool

	// The table structure has tbody as a direct child, or tr elements as direct children
	for child := table.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		switch child.Data {
		case "tbody":
			releases, rows = s.ProcessTbody(child, releases, &foundHeader)
		case "thead":
			releases = s.ProcessThead(child)
			foundHeader = true
		case "tr":
			releases, rows = s.ProcessTr(child, releases, rows, &foundHeader)
		}
	}

	// Parse each row to extract architecture and statuses
	for _, row := range rows {
		s.parseMatrixRow(row, releases, results)
	}
}

// ProcessTbody processes tbody element and extracts header and data rows
func (s *Scraper) ProcessTbody(tbody *html.Node, releases []string, foundHeader *bool) ([]string, []*html.Node) {
	var rows []*html.Node

	for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type == html.ElementNode && tr.Data == "tr" {
			// Check if this is the header row
			if !*foundHeader && s.isHeaderRow(tr) {
				releases = s.extractReleaseHeaders(tr)
				*foundHeader = true
			} else if *foundHeader {
				// This is a data row
				rows = append(rows, tr)
			}
		}
	}

	return releases, rows
}

// ProcessThead processes thead element and extracts release names
func (s *Scraper) ProcessThead(thead *html.Node) []string {
	for tr := thead.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type == html.ElementNode && tr.Data == "tr" {
			return s.extractReleaseHeaders(tr)
		}
	}
	return []string{}
}

// ProcessTr processes a direct tr element as either header or data row
func (s *Scraper) ProcessTr(tr *html.Node, releases []string, rows []*html.Node, foundHeader *bool) ([]string, []*html.Node) {
	if !*foundHeader && s.isHeaderRow(tr) {
		releases = s.extractReleaseHeaders(tr)
		*foundHeader = true
	} else if *foundHeader {
		rows = append(rows, tr)
	}
	return releases, rows
}

// findMatrixTable finds the main results table in the document
func (s *Scraper) findMatrixTable(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "table" {
		// Check if this table has the "table" class and expected structure
		hasTableClass := false
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, "table") {
				hasTableClass = true
				break
			}
		}

		if hasTableClass {
			// Verify it has the expected content (release names and architectures)
			tableText := getNodeText(n)
			hasReleases := strings.Contains(tableText, "focal") || strings.Contains(tableText, "jammy") ||
				strings.Contains(tableText, "noble") || strings.Contains(tableText, "questing")
			hasArchs := strings.Contains(tableText, "amd64") || strings.Contains(tableText, "arm64")

			if hasReleases && hasArchs {
				return n
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := s.findMatrixTable(child); result != nil {
			return result
		}
	}

	return nil
}

// isHeaderRow checks if a row is a header row
func (s *Scraper) isHeaderRow(tr *html.Node) bool {
	for child := tr.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "th" {
			return true
		}
	}
	return false
}

// extractReleaseHeaders extracts release names from the header row
func (s *Scraper) extractReleaseHeaders(headerNode *html.Node) []string {
	var releases []string

	// Find the tr element if we're in thead
	var headerRow *html.Node
	if headerNode.Data == "thead" {
		for child := headerNode.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "tr" {
				headerRow = child
				break
			}
		}
	} else if headerNode.Data == "tr" {
		headerRow = headerNode
	}

	if headerRow == nil {
		return releases
	}

	// Extract text from th or td elements
	for cell := headerRow.FirstChild; cell != nil; cell = cell.NextSibling {
		if cell.Type == html.ElementNode && (cell.Data == "th" || cell.Data == "td") {
			text := strings.TrimSpace(getNodeText(cell))
			releases = append(releases, text)
		}
	}

	return releases
}

// parseMatrixRow parses a single row of the matrix table
func (s *Scraper) parseMatrixRow(tr *html.Node, releases []string, results *PackageResults) {
	var cells []*html.Node
	var architecture string

	// Collect all cells
	cellIndex := 0
	for td := tr.FirstChild; td != nil; td = td.NextSibling {
		if td.Type == html.ElementNode && (td.Data == "td" || td.Data == "th") {
			if cellIndex == 0 {
				// First cell is the architecture
				architecture = strings.TrimSpace(getNodeText(td))
			} else {
				cells = append(cells, td)
			}
			cellIndex++
		}
	}

	// Skip if we don't have an architecture
	if len(strings.TrimSpace(architecture)) == 0 || len(cells) == 0 {
		return
	}

	// Parse each cell (one per release)
	for i, cell := range cells {
		if i >= len(releases) {
			break
		}

		release := releases[i]
		if len(strings.TrimSpace(release)) == 0 {
			continue
		}

		status := s.extractStatusFromCell(cell)
		if len(strings.TrimSpace(status)) == 0 {
			continue
		}

		test := TestResult{
			Package:      results.Package,
			Architecture: architecture,
			Release:      release,
			Status:       status,
		}

		// Try to extract link to detailed results
		if link := s.extractLink(cell); len(link) > 0 {
			// Make the URL absolute
			if !strings.HasPrefix(link, "http") {
				test.LogURL = fmt.Sprintf("%s/%s", s.BaseURL, link)
			} else {
				test.LogURL = link
			}
		}

		results.Tests = append(results.Tests, test)
	}
}

// extractStatusFromCell extracts the status text and emoji from a table cell
func (s *Scraper) extractStatusFromCell(cell *html.Node) string {
	text := strings.TrimSpace(getNodeText(cell))

	// Normalize the status text
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// Handle multiple spaces
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// extractLink extracts href from anchor tags
func (s *Scraper) extractLink(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "a" {
			for _, attr := range c.Attr {
				if attr.Key == "href" {
					return attr.Val
				}
			}
		}
	}
	return ""
}

// getNodeText extracts all text content from a node
func getNodeText(n *html.Node) string {
	var text strings.Builder

	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(n)
	return text.String()
}

// ReportErrors formats and returns a string with all errors found
func (r *PackageResults) ReportErrors() string {
	if len(r.Errors) == 0 {
		return fmt.Sprintf("No errors found for package: %s", r.Package)
	}

	var report strings.Builder
	report.WriteString(fmt.Sprintf("Found %d errors for package: %s\n\n", len(r.Errors), r.Package))

	for i, err := range r.Errors {
		report.WriteString(fmt.Sprintf("Error %d:\n", i+1))
		report.WriteString(fmt.Sprintf("\tStatus: %s\n", err.Status))
		if len(err.Release) > 0 {
			report.WriteString(fmt.Sprintf("\tRelease: %s\n", err.Release))
		}
		if len(err.Architecture) > 0 {
			report.WriteString(fmt.Sprintf("\tArchitecture: %s\n", err.Architecture))
		}
		if len(err.Duration) > 0 {
			report.WriteString(fmt.Sprintf("\tDuration: %s\n", err.Duration))
		}
		if len(err.Trigger) > 0 {
			report.WriteString(fmt.Sprintf("\tTrigger: %s\n", err.Trigger))
		}
		if len(err.LogURL) > 0 {
			report.WriteString(fmt.Sprintf("\tDetails: %s\n", err.LogURL))
		}
		report.WriteString("\n")
	}

	return report.String()
}
