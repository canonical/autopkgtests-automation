package autopkgtestclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// TriggerResult represents the result of triggering an autopkgtest
type TriggerResult struct {
	UUID       string // Test UUID
	ResultURL  string // URL to view test results
	HistoryURL string // URL to view result history
	Package    string // Package name
	Release    string // Ubuntu release
	Arch       string // Architecture
	Triggers   string // Trigger string used
	Requester  string // Username that requested the test
}

// TestStatus represents the status of a running test
type TestStatus struct {
	UUID      string    // Test UUID
	Status    string    // "queued", "running", "pass", "fail", "neutral", "tmpfail", "unknown"
	StartTime time.Time // When the test started (if available)
	Duration  string    // Test duration (if completed)
	LogURL    string    // URL to test logs
}

// AuthMethod defines how to authenticate with autopkgtest.ubuntu.com
type AuthMethod int

const (
	// AuthCookie uses existing session cookies (from browser or previous auth)
	AuthCookie AuthMethod = iota
	// AuthInteractive requires user to authenticate in their browser first
	AuthInteractive
)

// Client handles authenticated requests to autopkgtest.ubuntu.com
type Client struct {
	httpClient *http.Client
	baseURL    string
	authMethod AuthMethod
}

// ClientOption configures the Client
type ClientOption func(*Client)

// WithCookies configures the client to use specific cookies for authentication
func WithCookies(cookies []*http.Cookie) ClientOption {
	return func(c *Client) {
		if jar, ok := c.httpClient.Jar.(*cookiejar.Jar); ok {
			u, _ := url.Parse(c.baseURL)
			jar.SetCookies(u, cookies)
		}
	}
}

// WithAuthMethod sets the authentication method
func WithAuthMethod(method AuthMethod) ClientOption {
	return func(c *Client) {
		c.authMethod = method
	}
}

// NewClient creates a new autopkgtest client
func NewClient(opts ...ClientOption) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects, but track if we hit login
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				return nil
			},
		},
		baseURL:    "https://autopkgtest.ubuntu.com",
		authMethod: AuthInteractive,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// TriggerTest attempts to trigger an autopkgtest
// Returns TriggerResult if successful, or an error if authentication is needed or request failed
func (c *Client) TriggerTest(triggerURL string) (*TriggerResult, error) {
	resp, err := c.httpClient.Get(triggerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger test: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	bodyStr := string(body)

	// Check for success response first
	if strings.Contains(bodyStr, "Test request submitted") {
		result := &TriggerResult{}

		// Extract UUID - handle both plain text and HTML formats
		// Plain text: "UUID\n    uuid-value"
		// HTML: "<dt>UUID</dt>\n<dd>uuid-value</dd>"
		uuidRegex := regexp.MustCompile(`(?:UUID\s*\n\s*|<dt>UUID</dt>\s*\n\s*<dd>)([a-f0-9-]{36})`)
		if matches := uuidRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.UUID = matches[1]
		}

		// Extract Result URL
		resultURLRegex := regexp.MustCompile(`(?:Result url\s*\n\s*|<dt>Result url</dt>\s*\n\s*<dd>(?:<a[^>]*>)?)([^<\s]+)`)
		if matches := resultURLRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.ResultURL = matches[1]
		}

		// Extract History URL
		historyURLRegex := regexp.MustCompile(`(?:Result history\s*\n\s*|<dt>Result history</dt>\s*\n\s*<dd>(?:<a[^>]*>)?)([^<\s]+)`)
		if matches := historyURLRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.HistoryURL = matches[1]
		}

		// Extract package, release, arch
		packageRegex := regexp.MustCompile(`(?:package\s*\n\s*|<dt>package</dt>\s*\n\s*<dd>)([^<\s]+)`)
		if matches := packageRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.Package = matches[1]
		}

		releaseRegex := regexp.MustCompile(`(?:release\s*\n\s*|<dt>release</dt>\s*\n\s*<dd>)([^<\s]+)`)
		if matches := releaseRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.Release = matches[1]
		}

		archRegex := regexp.MustCompile(`(?:arch\s*\n\s*|<dt>arch</dt>\s*\n\s*<dd>)([^<\s]+)`)
		if matches := archRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.Arch = matches[1]
		}

		// Extract requester
		requesterRegex := regexp.MustCompile(`(?:requester\s*\n\s*|<dt>requester</dt>\s*\n\s*<dd>)([^<\s]+)`)
		if matches := requesterRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.Requester = matches[1]
		}

		// Extract triggers
		triggersRegex := regexp.MustCompile(`(?:triggers\s*\n\s*|<dt>triggers</dt>\s*\n\s*<dd>)(.+?)(?:\n|</dd>)`)
		if matches := triggersRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			result.Triggers = matches[1]
		}

		if result.UUID == "" {
			return nil, fmt.Errorf("test submission response parsed but UUID not found")
		}

		return result, nil
	}

	// Check for invalid request error (check this before auth check)
	if strings.Contains(bodyStr, "You submitted an invalid request") {
		// Check for specific "Test already running" error
		if strings.Contains(bodyStr, "Test already running") {
			return nil, fmt.Errorf("test already running for this package/release/arch combination")
		}

		// Extract the error message
		// Pattern: <p>You submitted an invalid request: error message</p>
		errorRegex := regexp.MustCompile(`<p>You submitted an invalid request:\s*([^<]+)</p>`)
		if matches := errorRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
			errorMsg := strings.TrimSpace(matches[1])
			return nil, fmt.Errorf("invalid request: %s", errorMsg)
		}
		return nil, fmt.Errorf("invalid request (details not available)")
	}

	// Check if we need authentication
	// Look for redirect to login page or login prompt (but not "Logout" which means we're authenticated)
	if strings.Contains(resp.Request.URL.String(), "/login") ||
		(strings.Contains(bodyStr, "login") && !strings.Contains(bodyStr, "Logout")) {
		return nil, fmt.Errorf("authentication required: please authenticate first")
	}

	// Unknown response
	return nil, fmt.Errorf("unexpected response from server")
}

// GetTestStatus checks the status of a test by UUID
func (c *Client) GetTestStatus(uuid string) (*TestStatus, error) {
	resultURL := fmt.Sprintf("%s/run/%s", c.baseURL, uuid)

	resp, err := c.httpClient.Get(resultURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get test status: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	bodyStr := string(body)

	status := &TestStatus{
		UUID:   uuid,
		LogURL: resultURL,
	}

	// Determine test status based on the Result field in the page
	// Use regex to match the Result row in the table to avoid false positives from navigation menu
	resultRegex := regexp.MustCompile(`(?i)\|\s*Result\s*\|[^|]*\|`)
	if resultMatch := resultRegex.FindString(bodyStr); resultMatch != "" {
		// Check the actual result value - check for fail first to avoid substring issues
		// Status values can be: pass, fail, neutral, tmpfail
		if strings.Contains(resultMatch, "tmpfail") || strings.Contains(resultMatch, "⚠ tmpfail") {
			status.Status = "tmpfail"
		} else if strings.Contains(resultMatch, "✖ fail") || strings.Contains(resultMatch, "fail") {
			status.Status = "fail"
		} else if strings.Contains(resultMatch, "✔ pass") || strings.Contains(resultMatch, "pass") {
			status.Status = "pass"
		} else if strings.Contains(resultMatch, "neutral") || strings.Contains(resultMatch, "😐neutral") {
			status.Status = "neutral"
		}
	} else {
		// Fallback to checking page content for in-progress states
		if strings.Contains(bodyStr, "In progress") {
			status.Status = "running"
		} else if strings.Contains(bodyStr, "Queued") {
			status.Status = "queued"
		} else {
			status.Status = "unknown"
		}
	}

	// Try to extract duration if test is complete
	// Match both table format: | Duration | 1h 20m 25s |
	// And plain text format: Duration: 1h 20m 25s
	durationRegex := regexp.MustCompile(`(?:Duration[:\s]*\|?\s*)([^|\n]+)`)
	if matches := durationRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
		status.Duration = strings.TrimSpace(matches[1])
	}

	return status, nil
}

// FindRunningTest attempts to find the UUID of a currently running test
// by checking the running tests page for the given package/release/arch combination
func (c *Client) FindRunningTest(packageName, release, arch string) (string, error) {
	// Try the main package page first (without release/arch) - shows running tests
	packagesURL := fmt.Sprintf("%s/packages/%s", c.baseURL, packageName)

	resp, err := c.httpClient.Get(packagesURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch packages page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	bodyStr := string(body)

	// Look for running tests - they have "Running for:" text near them
	// Split into lines and search for UUID patterns near "Running for:"
	// The HTML structure may have <th>UUID:</th> on one line and <td>uuid-value</td> on the next
	uuidHeaderRegex := regexp.MustCompile(`<th>UUID:</th>`)
	uuidValueRegex := regexp.MustCompile(`<td>([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})</td>`)
	runningForRegex := regexp.MustCompile(`Running for:`)

	lines := strings.Split(bodyStr, "\n")
	for i, line := range lines {
		// Look for "Running for:" indicator
		if runningForRegex.MatchString(line) {
			// Search backwards for UUID in the previous ~20 lines
			for j := i - 1; j >= 0 && j >= i-20; j-- {
				// Check if this line has <th>UUID:</th>
				if uuidHeaderRegex.MatchString(lines[j]) {
					// UUID value might be on the same line or the next line
					var uuid string
					uuidLine := j

					// Try current line first
					if matches := uuidValueRegex.FindStringSubmatch(lines[j]); len(matches) > 1 {
						uuid = matches[1]
					} else if j+1 < len(lines) {
						// Try next line
						if matches := uuidValueRegex.FindStringSubmatch(lines[j+1]); len(matches) > 1 {
							uuid = matches[1]
							uuidLine = j + 1
						}
					}

					if uuid == "" {
						continue
					}

					// Check if this test matches our release and arch
					// Look for Release: and Architecture: in nearby lines
					// The format is: <th>Release:</th> followed by <td>value</td>
					// These fields are typically 30-50 lines before the UUID
					contextStart := uuidLine - 50
					if contextStart < 0 {
						contextStart = 0
					}
					contextEnd := uuidLine + 10
					if contextEnd >= len(lines) {
						contextEnd = len(lines) - 1
					}

					context := strings.Join(lines[contextStart:contextEnd], "\n")
					// Match the actual HTML structure: <th>Release:</th> possibly followed by newlines then <td>noble</td>
					// Use (?s) for dot to match newlines, and \s* to allow whitespace/newlines
					releasePattern := fmt.Sprintf(`<th>Release:</th>[\s\n]*<td>%s</td>`, release)
					archPattern := fmt.Sprintf(`<th>Architecture:</th>[\s\n]*<td>%s</td>`, arch)

					releaseMatch, _ := regexp.MatchString(releasePattern, context)
					archMatch, _ := regexp.MatchString(archPattern, context)

					if releaseMatch && archMatch {
						// Verify it's actually running
						status, err := c.GetTestStatus(uuid)
						if err == nil && (status.Status == "running" || status.Status == "queued") {
							return uuid, nil
						}
					}
				}
			}
		}
	}

	// Fallback: try the running page
	runningURL := fmt.Sprintf("%s/running", c.baseURL)
	resp, err = c.httpClient.Get(runningURL)
	if err != nil {
		return "", fmt.Errorf("test not found on running page")
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read running page: %w", err)
	}

	bodyStr = string(body)

	// Look for matching package/release/arch in running tests
	if strings.Contains(bodyStr, packageName) {
		allUUIDRegex := regexp.MustCompile(`/run/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`)
		matches := allUUIDRegex.FindAllStringSubmatch(bodyStr, -1)
		for _, match := range matches {
			uuid := match[1]
			// Check if this UUID is for our package/release/arch
			status, err := c.GetTestStatus(uuid)
			if err == nil && (status.Status == "running" || status.Status == "queued") {
				return uuid, nil
			}
		}
	}

	return "", fmt.Errorf("no running test found for %s/%s/%s", packageName, release, arch)
}

// WaitForCompletion polls the test status until it completes or times out
// pollInterval: how often to check status (e.g., 30s)
// timeout: maximum time to wait (e.g., 2h)
func (c *Client) WaitForCompletion(pkg, uuid string, pollInterval, timeout time.Duration) (*TestStatus, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := c.GetTestStatus(uuid)
			if err != nil {
				return nil, err
			}

			// Check if test is complete
			if status.Status == "pass" || status.Status == "fail" || status.Status == "neutral" || status.Status == "tmpfail" {
				return status, nil
			}

		case <-timeoutTimer.C:
			// Timeout reached, return last known status
			status, err := c.GetTestStatus(uuid)
			if err != nil {
				return nil, fmt.Errorf("timeout reached and failed to get final status: %w", err)
			}
			return nil, fmt.Errorf("timeout reached after %v (last status: %s)", timeout, status.Status)
		}
	}
}

// GetCookies returns the current session cookies
func (c *Client) GetCookies() []*http.Cookie {
	u, _ := url.Parse(c.baseURL)
	return c.httpClient.Jar.Cookies(u)
}
