package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	autopkgtestclient "github.com/canonical/autopkgtest-automation/internal/autopkgtest-client"
	"github.com/canonical/autopkgtest-automation/internal/scraper"
	triggerlinkgenerator "github.com/canonical/autopkgtest-automation/internal/trigger-link-generator"
)

const (
	version = "0.1.0"
)

func main() {
	// Define subcommands
	checkCmd := flag.NewFlagSet("check", flag.ExitOnError)
	generateLinkCmd := flag.NewFlagSet("generate-trigger-link", flag.ExitOnError)
	triggerCmd := flag.NewFlagSet("trigger", flag.ExitOnError)
	versionCmd := flag.NewFlagSet("version", flag.ExitOnError)

	// Check command flags
	checkPackage := checkCmd.String("package", "", "Package name to check (required)")
	checkVerbose := checkCmd.Bool("verbose", false, "Show all test results, not just errors")
	checkRelease := checkCmd.String("release", "", "Filter by specific release (optional, e.g., noble, jammy)")
	checkArch := checkCmd.String("arch", "", "Filter by specific architecture (optional, e.g., amd64, arm64)")

	// Generate-trigger-link command flags
	genPackage := generateLinkCmd.String("package", "", "Package name to generate trigger link for (required)")
	genVersion := generateLinkCmd.String("version", "", "Package version (optional)")
	genArch := generateLinkCmd.String("arch", "", "Comma-separated list of architectures (optional, e.g., amd64,arm64)")
	genSuite := generateLinkCmd.String("suite", "", "Ubuntu suite/release (required, e.g., noble, mantic, jammy)")
	genTrigger := generateLinkCmd.String("trigger", "", "Custom trigger string (optional, overrides package/version)")
	genPPA := generateLinkCmd.String("ppa", "", "PPA to test against (optional, format: user/ppa-name)")
	genAllProposed := generateLinkCmd.Bool("all-proposed", false, "Install all packages from proposed pocket")

	// Trigger command flags (will use authentication)
	triggerPackage := triggerCmd.String("package", "", "Package name to trigger test for (required)")
	triggerVersion := triggerCmd.String("version", "", "Package version (optional)")
	triggerArch := triggerCmd.String("arch", "", "Comma-separated list of architectures (optional, e.g., amd64,arm64)")
	triggerSuite := triggerCmd.String("suite", "", "Ubuntu suite/release (required, e.g., noble, mantic, jammy)")
	triggerTrigger := triggerCmd.String("trigger", "", "Custom trigger string (optional, overrides package/version)")
	triggerPPA := triggerCmd.String("ppa", "", "PPA to test against (optional, format: user/ppa-name)")
	triggerAllProposed := triggerCmd.Bool("all-proposed", false, "Install all packages from proposed pocket")
	triggerCredentials := triggerCmd.String("credentials", "", "Path to cookie file with Launchpad session (optional)")
	triggerWait := triggerCmd.Bool("wait", false, "Wait for test completion")
	triggerTimeout := triggerCmd.Duration("timeout", 2*time.Hour, "Maximum time to wait for test completion")
	triggerPollInterval := triggerCmd.Duration("poll-interval", 60*time.Second, "How often to check test status")

	// Parse command line
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "check":
		checkCmd.Parse(os.Args[2:])
		if *checkPackage == "" {
			fmt.Println("Error: -package flag is required")
			checkCmd.PrintDefaults()
			os.Exit(1)
		}
		handleCheck(*checkPackage, *checkVerbose, *checkRelease, *checkArch)

	case "generate-trigger-link":
		generateLinkCmd.Parse(os.Args[2:])
		if *genPackage == "" {
			fmt.Println("Error: -package flag is required")
			generateLinkCmd.PrintDefaults()
			os.Exit(1)
		}
		if *genSuite == "" {
			fmt.Println("Error: -suite flag is required")
			generateLinkCmd.PrintDefaults()
			os.Exit(1)
		}

		var archs []string
		if *genArch != "" {
			archs = strings.Split(*genArch, ",")
			for i := range archs {
				archs[i] = strings.TrimSpace(archs[i])
			}
		}

		// Parse comma-separated triggers into a slice
		var triggers []string
		if *genTrigger != "" {
			triggers = strings.Split(*genTrigger, ",")
			for i := range triggers {
				triggers[i] = strings.TrimSpace(triggers[i])
			}
		}

		handleGenerateTriggerLink(*genPackage, *genVersion, *genSuite, triggers, *genPPA, *genAllProposed, archs)

	case "trigger":
		triggerCmd.Parse(os.Args[2:])
		if *triggerPackage == "" {
			fmt.Println("Error: -package flag is required")
			triggerCmd.PrintDefaults()
			os.Exit(1)
		}
		if *triggerSuite == "" {
			fmt.Println("Error: -suite flag is required")
			triggerCmd.PrintDefaults()
			os.Exit(1)
		}

		var archs []string
		if *triggerArch != "" {
			archs = strings.Split(*triggerArch, ",")
			for i := range archs {
				archs[i] = strings.TrimSpace(archs[i])
			}
		}

		// Parse comma-separated triggers into a slice
		var triggers []string
		if *triggerTrigger != "" {
			triggers = strings.Split(*triggerTrigger, ",")
			for i := range triggers {
				triggers[i] = strings.TrimSpace(triggers[i])
			}
		}

		handleTrigger(*triggerPackage, *triggerVersion, *triggerSuite, triggers, *triggerPPA, *triggerAllProposed, *triggerCredentials, *triggerWait, *triggerTimeout, *triggerPollInterval, archs)

	case "version":
		versionCmd.Parse(os.Args[2:])
		fmt.Printf("autopkgtest-cli version %s\n", version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print("autopkgtest-cli - Autopkgtest automation tool\n\n" +
		"Usage:\n" +
		"\tautopkgtest-cli <command> [flags]\n\n" +
		"Commands:\n" +
		"\tcheck\t\t\tCheck autopkgtest results for a package\n" +
		"\tgenerate-trigger-link\tGenerate autopkgtest trigger URL(s)\n" +
		"\ttrigger\t\t\tTrigger autopkgtest with authentication\n" +
		"\tversion\t\t\tShow version information\n" +
		"\thelp\t\t\tShow this help message\n\n" +
		"Check command:\n" +
		"\tautopkgtest-cli check -package <name> [-verbose] [-release <release>] [-arch <arch>]\n\n" +
		"Check options:\n" +
		"\t-package string      Package name (required)\n" +
		"\t-verbose             Show all test results, not just errors\n" +
		"\t-release string      Filter by release (optional, e.g., noble, jammy)\n" +
		"\t-arch string         Filter by architecture (optional, e.g., amd64, arm64)\n\n" +
		"Generate-trigger-link command:\n" +
		"\tautopkgtest-cli generate-trigger-link -package <name> -suite <suite> [options]\n\n" +
		"Generate-trigger-link options:\n" +
		"\t-package string      Package name (required)\n" +
		"\t-suite string        Ubuntu release (required, e.g., noble, mantic, jammy)\n" +
		"\t-arch string         Architectures (optional, comma-separated: amd64,arm64)\n" +
		"\t-version string      Package version (optional)\n" +
		"\t-trigger string      Custom trigger (optional, comma-separated for multiple)\n" +
		"\t-ppa string          PPA to test (optional, e.g., user/ppa-name)\n" +
		"\t-all-proposed        Use all packages from proposed pocket\n\n" +
		"Trigger command (with authentication - skeleton):\n" +
		"\tautopkgtest-cli trigger -package <name> -suite <suite> [options]\n\n" +
		"Trigger options:\n" +
		"\t-package string      Package name (required)\n" +
		"\t-suite string        Ubuntu release (required, e.g., noble, mantic, jammy)\n" +
		"\t-arch string         Architectures (optional, comma-separated: amd64,arm64)\n" +
		"\t-version string      Package version (optional)\n" +
		"\t-trigger string      Custom trigger (optional, comma-separated for multiple)\n" +
		"\t-ppa string          PPA to test (optional, e.g., user/ppa-name)\n" +
		"\t-all-proposed        Use all packages from proposed pocket\n" +
		"\t-credentials string  Path to cookie file with Launchpad session (optional)\n" +
		"\t-wait                Wait for test completion\n" +
		"\t-timeout duration    Maximum time to wait (default: 2h)\n" +
		"\t-poll-interval duration  How often to check status (default: 30s)\n\n" +
		"Examples:\n" +
		"\tautopkgtest-cli check -package ovn\n" +
		"\tautopkgtest-cli check -package ovn -verbose\n" +
		"\tautopkgtest-cli check -package ovn -release noble -arch amd64\n" +
		"\tautopkgtest-cli generate-trigger-link -package ovn -suite noble\n" +
		"\tautopkgtest-cli generate-trigger-link -package ovn -suite noble -arch amd64,arm64\n" +
		"\tautopkgtest-cli generate-trigger-link -package myapp -suite noble -trigger systemd/259-1ubuntu3,dhcpcd/1:10.3.0-7\n" +
		"\tautopkgtest-cli trigger -package ovn -suite noble -credentials ~/.autopkgtest-cookies\n" +
		"\tautopkgtest-cli trigger -package ovn -suite noble --wait --timeout 1h\n" +
		"\tautopkgtest-cli trigger -package myapp -suite noble -trigger systemd/259-1ubuntu3,dhcpcd/1:10.3.0-7 -credentials ~/.autopkgtest-cookies\n")
}

func handleCheck(packageName string, verbose bool, release, arch string) {
	fmt.Printf("Checking autopkgtest results for package: %s\n", packageName)
	if release != "" || arch != "" {
		fmt.Print("Filters: ")
		if release != "" {
			fmt.Printf("release=%s ", release)
		}
		if arch != "" {
			fmt.Printf("arch=%s", arch)
		}
		fmt.Println()
	}
	fmt.Println()

	s := scraper.NewScraper()
	var filter *scraper.Filter
	if release != "" || arch != "" {
		filter = &scraper.Filter{
			Release:      release,
			Architecture: arch,
		}
	}
	results, err := s.FetchPackageResultsFiltered(packageName, filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching results: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Total tests found: %d\n", len(results.Tests))
		fmt.Println()

		if len(results.Tests) > 0 {
			fmt.Println("All test results:")
			for i, test := range results.Tests {
				fmt.Printf("\nTest %d:\n", i+1)
				fmt.Printf("\tStatus: %s\n", test.Status)
				if test.Release != "" {
					fmt.Printf("\tRelease: %s\n", test.Release)
				}
				if test.Architecture != "" {
					fmt.Printf("\tArchitecture: %s\n", test.Architecture)
				}
				if test.Duration != "" {
					fmt.Printf("\tDuration: %s\n", test.Duration)
				}
				if test.Trigger != "" {
					fmt.Printf("\tTrigger: %s\n", test.Trigger)
				}
				if test.LogURL != "" {
					fmt.Printf("\tDetails: %s\n", test.LogURL)
				}
			}
			fmt.Println()
		}
	}

	// Always show error report
	report := results.ReportErrors()
	fmt.Println(report)

	// Exit with error code if errors were found
	if len(results.Errors) > 0 {
		os.Exit(1)
	}
}

func handleGenerateTriggerLink(packageName, version, suite string, triggers []string, ppa string, allProposed bool, archs []string) {
	if packageName == "" {
		fmt.Fprintln(os.Stderr, "Error: -package is required")
		os.Exit(1)
	}

	if suite == "" {
		fmt.Fprintln(os.Stderr, "Error: -suite is required")
		os.Exit(1)
	}

	req := &triggerlinkgenerator.LinkRequest{
		Package:       packageName,
		Version:       version,
		Suite:         suite,
		Triggers:      triggers,
		PPA:           ppa,
		AllProposed:   allProposed,
		Architectures: archs,
	}

	gen := triggerlinkgenerator.NewGenerator()
	resp, err := gen.GenerateLinks(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating trigger links: %v\n", err)
		os.Exit(1)
	}

	if len(resp.URLs) == 0 {
		fmt.Println("No links generated.")
		return
	}

	fmt.Printf("Generated autopkgtest trigger URL(s) for package: %s\n\n", packageName)
	fmt.Println("Visit the following URL(s) in your browser:")
	fmt.Println("(You must be logged into Launchpad with appropriate permissions)")
	fmt.Println()

	for _, link := range resp.URLs {
		fmt.Println(link)
	}
}

// handleTrigger triggers autopkgtest with authentication
func handleTrigger(packageName, version, suite string, triggers []string, ppa string, allProposed bool, credentials string, wait bool, timeout, pollInterval time.Duration, archs []string) {
	fmt.Println("=== Autopkgtest Trigger ===")
	fmt.Println()

	if packageName == "" {
		fmt.Fprintln(os.Stderr, "Error: -package is required")
		os.Exit(1)
	}

	if suite == "" {
		fmt.Fprintln(os.Stderr, "Error: -suite is required")
		os.Exit(1)
	}

	// Generate the trigger URLs
	req := &triggerlinkgenerator.LinkRequest{
		Package:       packageName,
		Version:       version,
		Suite:         suite,
		Triggers:      triggers,
		PPA:           ppa,
		AllProposed:   allProposed,
		Architectures: archs,
	}

	gen := triggerlinkgenerator.NewGenerator()
	resp, err := gen.GenerateLinks(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating trigger links: %v\n", err)
		os.Exit(1)
	}

	if len(resp.URLs) == 0 {
		fmt.Println("No links to trigger.")
		return
	}

	// Create autopkgtest client
	var clientOpts []autopkgtestclient.ClientOption

	if credentials != "" {
		cookies, err := loadCookiesFromFile(credentials)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load cookies from %s: %v\n", credentials, err)
			fmt.Fprintf(os.Stderr, "Will attempt to trigger without authentication (may fail)\n\n")
		} else {
			fmt.Printf("Loaded %d cookie(s) from %s\n\n", len(cookies), credentials)
			clientOpts = append(clientOpts, autopkgtestclient.WithCookies(cookies))
		}
	}

	client, err := autopkgtestclient.NewClient(clientOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating client: %v\n", err)
		os.Exit(1)
	}

	// Trigger tests for each URL
	var results []*autopkgtestclient.TriggerResult
	for i, triggerURL := range resp.URLs {
		if len(resp.URLs) > 1 {
			fmt.Printf("[%d/%d] Triggering test...\n", i+1, len(resp.URLs))
		} else {
			fmt.Println("Triggering test...")
		}

		result, err := client.TriggerTest(triggerURL)
		if err != nil {
			if strings.Contains(err.Error(), "authentication required") {
				fmt.Fprintf(os.Stderr, "\nAuthentication required!\n\n")
				fmt.Fprintf(os.Stderr, "Please authenticate in your browser:\n")
				fmt.Fprintf(os.Stderr, "\t1. Visit: https://autopkgtest.ubuntu.com/login\n")
				fmt.Fprintf(os.Stderr, "\t2. Log in with your Launchpad credentials\n")
				fmt.Fprintf(os.Stderr, "\t3. Export your session cookies and save to a file\n")
				fmt.Fprintf(os.Stderr, "\t4. Retry with: -credentials <cookie-file>\n\n")
				fmt.Fprintf(os.Stderr, "Alternatively, open the URL manually in your browser:\n")
				fmt.Fprintf(os.Stderr, "  %s\n\n", triggerURL)
				os.Exit(1)
			} else if strings.Contains(err.Error(), "already running") {
				// Try to find the running test
				arch := extractArchFromURL(triggerURL)
				fmt.Printf("⚠ Test already running for %s/%s/%s\n", packageName, suite, arch)
				fmt.Printf("\tAttempting to find running test UUID...\n")

				uuid, err := client.FindRunningTest(packageName, suite, arch)
				if err != nil {
					fmt.Fprintf(os.Stderr, "\tCould not find running test UUID: %v\n", err)
					fmt.Fprintf(os.Stderr, "\tCheck status manually at: https://autopkgtest.ubuntu.com/packages/%s/%s/%s\n\n", packageName, suite, arch)
					continue
				}

				// Create a fake result for the running test so we can monitor it
				result = &autopkgtestclient.TriggerResult{
					UUID:       uuid,
					ResultURL:  fmt.Sprintf("https://autopkgtest.ubuntu.com/run/%s", uuid),
					HistoryURL: fmt.Sprintf("https://autopkgtest.ubuntu.com/packages/%s/%s/%s", packageName, suite, arch),
					Package:    packageName,
					Release:    suite,
					Arch:       arch,
				}

				fmt.Printf("\t✓ Found running test!\n")
				fmt.Printf("\tUUID:    %s\n", result.UUID)
				fmt.Printf("\tResults: %s\n", result.ResultURL)
				fmt.Println()
			} else if strings.Contains(err.Error(), "invalid request") {
				fmt.Fprintf(os.Stderr, "✗ %v\n", err)
				os.Exit(1)
			} else {
				fmt.Fprintf(os.Stderr, "Error triggering test: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Printf("✓ Test triggered successfully!\n")
			fmt.Printf("\tUUID:     %s\n", result.UUID)
			fmt.Printf("\tPackage:  %s\n", result.Package)
			fmt.Printf("\tRelease:  %s\n", result.Release)
			fmt.Printf("\tArch:     %s\n", result.Arch)
			fmt.Printf("\tResults:  %s\n", result.ResultURL)
			fmt.Println()
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		fmt.Println("No tests were triggered.")
		os.Exit(1)
	}

	// Wait for completion if requested
	if wait {
		fmt.Printf("Waiting for test completion (timeout: %v, poll interval: %v)...\n\n", timeout, pollInterval)

		hasFailure := false
		for _, result := range results {
			fmt.Printf("Monitoring: %s [%s/%s]\n", result.Package, result.Release, result.Arch)
			fmt.Printf("UUID: %s\n", result.UUID)
			// Print the packages page URL where live logs can be viewed
			packagesURL := fmt.Sprintf("https://autopkgtest.ubuntu.com/packages/%s", result.Package)
			fmt.Printf("View logs: %s\n", packagesURL)
			fmt.Println("Waiting for test to complete...")
			fmt.Println()

			status, err := client.WaitForCompletion(result.Package, result.UUID, pollInterval, timeout)
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					fmt.Fprintf(os.Stderr, "⏱ Timeout reached. Test still running.\n")
					fmt.Fprintf(os.Stderr, "Check status at: %s\n\n", packagesURL)
					hasFailure = true
					continue
				}
				fmt.Fprintf(os.Stderr, "Error monitoring test: %v\n\n", err)
				hasFailure = true
				continue
			}

			fmt.Println("=== Test Complete ===")
			switch status.Status {
			case "pass":
				fmt.Printf("✓ PASS")
			case "fail":
				fmt.Printf("✗ FAIL")
				hasFailure = true
			case "neutral":
				fmt.Printf("○ NEUTRAL")
			default:
				fmt.Printf("? %s", strings.ToUpper(status.Status))
			}

			if status.Duration != "" {
				fmt.Printf(" (Duration: %s)", status.Duration)
			}
			fmt.Println()
			// Now print the result URL since the test is complete
			fmt.Printf("Results: %s\n\n", status.LogURL)
		}

		if hasFailure {
			fmt.Fprintln(os.Stderr, "One or more tests failed or timed out.")
			os.Exit(1)
		}
		fmt.Println("All tests completed successfully.")
	} else {
		fmt.Println("Tests triggered. Check status and logs at:")
		for _, result := range results {
			packagesURL := fmt.Sprintf("https://autopkgtest.ubuntu.com/packages/%s", result.Package)
			fmt.Printf("  • %s (%s/%s) - %s\n", result.Package, result.Release, result.Arch, packagesURL)
		}
		fmt.Println()
		fmt.Println("Tip: Use --wait flag to monitor test completion automatically.")
	}
}

// loadCookiesFromFile loads cookie value from a plain text file
func loadCookiesFromFile(filepath string) ([]*http.Cookie, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Trim whitespace from cookie value
	cookieValue := strings.TrimSpace(string(data))
	if cookieValue == "" {
		return nil, fmt.Errorf("cookie file is empty")
	}

	// Create session cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    cookieValue,
		Domain:   "autopkgtest.ubuntu.com",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	}

	return []*http.Cookie{cookie}, nil
}

// extractArchFromURL extracts architecture from trigger URL
func extractArchFromURL(url string) string {
	if strings.Contains(url, "arch=") {
		parts := strings.Split(url, "arch=")
		if len(parts) > 1 {
			arch := strings.Split(parts[1], "&")[0]
			return arch
		}
	}
	return "all"
}
