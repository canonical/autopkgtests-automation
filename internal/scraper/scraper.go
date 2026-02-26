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

	// Find the results table and parse it
	table := findResultsTable(doc)
	if table == nil {
		return results, nil
	}

	// Extract the header (release names) and data rows from the table
	releases, dataRows := extractTableStructure(table)

	// Build test results from each data row
	for _, row := range dataRows {
		s.parseDataRow(row, releases, results)
	}

	// Apply filters if provided
	if filter != nil {
		results.Tests = applyFilter(results.Tests, filter)
	}

	// Collect errors (tests with non-passing status)
	for _, test := range results.Tests {
		if !isPassingStatus(test.Status) {
			results.Errors = append(results.Errors, test)
		}
	}

	return results, nil
}

// applyFilter filters test results based on the provided criteria
func applyFilter(tests []TestResult, filter *Filter) []TestResult {
	if filter == nil {
		return tests
	}

	var filtered []TestResult
	for _, test := range tests {
		if filter.Release != "" && !strings.EqualFold(test.Release, filter.Release) {
			continue
		}
		if filter.Architecture != "" && !matchesArchitecture(test.Architecture, filter.Architecture) {
			continue
		}
		filtered = append(filtered, test)
	}

	return filtered
}

// matchesArchitecture checks whether arch matches the filter, which may be a
// single architecture ("amd64") or a comma-separated list ("amd64,arm64").
func matchesArchitecture(arch, filter string) bool {
	for _, f := range strings.Split(filter, ",") {
		if strings.EqualFold(strings.TrimSpace(f), arch) {
			return true
		}
	}
	return false
}

// isPassingStatus checks if a status indicates a passing test
func isPassingStatus(status string) bool {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	return normalizedStatus == "pass" || normalizedStatus == "✔ pass" ||
		normalizedStatus == "neutral" || normalizedStatus == "😐 neutral" ||
		strings.Contains(normalizedStatus, "pass")
}

// ---------------------------------------------------------------------------
// Table discovery
// ---------------------------------------------------------------------------

// findResultsTable locates the autopkgtest results table in the parsed HTML
// document. It looks for a <table> with a CSS class containing "table".
func findResultsTable(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "table" {
		if hasClass(n, "table") {
			return n
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if result := findResultsTable(child); result != nil {
			return result
		}
	}
	return nil
}

// hasClass checks whether an HTML node has a given CSS class.
func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" && strings.Contains(attr.Val, class) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Table structure extraction
// ---------------------------------------------------------------------------

// extractTableStructure walks the table node and returns the list of release
// names (column headers, excluding the leading architecture-label column) and
// the set of data <tr> nodes (one per architecture).
func extractTableStructure(table *html.Node) (releases []string, dataRows []*html.Node) {
	var headerFound bool

	// collectRows is called for each container that may hold <tr> elements
	// (the table itself, a <thead>, or a <tbody>).
	var collectRows func(container *html.Node)
	collectRows = func(container *html.Node) {
		for child := container.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode {
				continue
			}
			switch child.Data {
			case "thead":
				releases = extractReleaseNames(child)
				headerFound = true
			case "tbody":
				collectRows(child) // recurse into tbody
			case "tr":
				if !headerFound && isHeaderRow(child) {
					releases = extractReleaseNames(child)
					headerFound = true
				} else if headerFound {
					dataRows = append(dataRows, child)
				}
			}
		}
	}

	collectRows(table)
	return releases, dataRows
}

// isHeaderRow returns true when every cell in the row is a <th>.
// Data rows in the autopkgtest table use <th> only for the first cell
// (architecture name) and <td> for the rest, so a row that is entirely
// <th> elements is the header.
func isHeaderRow(tr *html.Node) bool {
	hasTH := false
	for child := tr.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.Data == "td" {
			return false // data rows have <td> cells
		}
		if child.Data == "th" {
			hasTH = true
		}
	}
	return hasTH
}

// extractReleaseNames returns the non-empty column headers from a header row
// or <thead>. The first cell in the header row is the empty corner cell above
// the architecture column and is intentionally skipped so that the returned
// slice aligns 1:1 with the data cells that follow the architecture cell in
// each data row.
func extractReleaseNames(node *html.Node) []string {
	// If we received a <thead>, drill down to its <tr>.
	tr := node
	if node.Data == "thead" {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.Data == "tr" {
				tr = child
				break
			}
		}
	}

	var releases []string
	first := true
	for cell := tr.FirstChild; cell != nil; cell = cell.NextSibling {
		if cell.Type == html.ElementNode && (cell.Data == "th" || cell.Data == "td") {
			if first {
				// Skip the leading corner cell (empty <th> above the arch column).
				first = false
				continue
			}
			text := strings.TrimSpace(getNodeText(cell))
			if text != "" {
				releases = append(releases, text)
			}
		}
	}
	return releases
}

// ---------------------------------------------------------------------------
// Row parsing
// ---------------------------------------------------------------------------

// parseDataRow extracts the architecture name and per-release statuses from a
// single data row and appends the results to results.Tests.
//
// The expected row layout is:
//
//	<tr>
//	  <th>amd64</th>            ← architecture
//	  <td class="pass">…</td>  ← releases[0]
//	  <td class="fail">…</td>  ← releases[1]
//	  …
//	</tr>
func (s *Scraper) parseDataRow(tr *html.Node, releases []string, results *PackageResults) {
	var architecture string
	var dataCells []*html.Node

	first := true
	for child := tr.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.Data != "td" && child.Data != "th" {
			continue
		}
		if first {
			architecture = strings.TrimSpace(getNodeText(child))
			first = false
			continue
		}
		dataCells = append(dataCells, child)
	}

	if architecture == "" || len(dataCells) == 0 {
		return
	}

	for i, cell := range dataCells {
		if i >= len(releases) {
			break
		}

		status := extractStatusFromCell(cell)
		if status == "" {
			continue
		}

		test := TestResult{
			Package:      results.Package,
			Architecture: architecture,
			Release:      releases[i],
			Status:       status,
		}

		if link := extractLink(cell); link != "" {
			if !strings.HasPrefix(link, "http") {
				test.LogURL = fmt.Sprintf("%s/%s", s.BaseURL, link)
			} else {
				test.LogURL = link
			}
		}

		results.Tests = append(results.Tests, test)
	}
}

// ---------------------------------------------------------------------------
// Cell helpers
// ---------------------------------------------------------------------------

// extractStatusFromCell returns the normalized status text from a table cell.
func extractStatusFromCell(cell *html.Node) string {
	text := strings.TrimSpace(getNodeText(cell))
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// extractLink returns the href value of the first <a> child of node.
func extractLink(n *html.Node) string {
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

// getNodeText extracts all text content from a node and its children.
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

// ---------------------------------------------------------------------------
// Reporting
// ---------------------------------------------------------------------------

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
