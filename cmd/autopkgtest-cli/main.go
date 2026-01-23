package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/canonical/autopkgtest-automation/internal/scraper"
	"github.com/canonical/autopkgtest-automation/internal/trigger"
)

const (
	version = "0.1.0"
)

func main() {
	// Define subcommands
	checkCmd := flag.NewFlagSet("check", flag.ExitOnError)
	triggerCmd := flag.NewFlagSet("trigger", flag.ExitOnError)
	versionCmd := flag.NewFlagSet("version", flag.ExitOnError)

	// Check command flags
	checkPackage := checkCmd.String("package", "", "Package name to check (required)")
	checkVerbose := checkCmd.Bool("verbose", false, "Show all test results, not just errors")

	// Trigger command flags
	triggerPackage := triggerCmd.String("package", "", "Package name to trigger test for (required)")
	triggerVersion := triggerCmd.String("version", "", "Package version (optional)")
	triggerArch := triggerCmd.String("arch", "", "Comma-separated list of architectures (optional)")
	triggerSuite := triggerCmd.String("suite", "", "Ubuntu suite (e.g., noble, mantic) (optional)")
	triggerTrigger := triggerCmd.String("trigger", "", "Trigger package (optional)")

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
		handleCheck(*checkPackage, *checkVerbose)

	case "trigger":
		triggerCmd.Parse(os.Args[2:])
		if *triggerPackage == "" {
			fmt.Println("Error: -package flag is required")
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

		handleTrigger(*triggerPackage, *triggerVersion, *triggerSuite, *triggerTrigger, archs)

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
		"\tcheck\t\tCheck autopkgtest results for a package\n" +
		"\ttrigger\t\tTrigger an autopkgtest for a package\n" +
		"\tversion\t\tShow version information\n" +
		"\thelp\t\tShow this help message\n\n" +
		"Check command:\n" +
		"\tautopkgtest-cli check -package <name> [-verbose]\n\n" +
		"Trigger command:\n" +
		"\tautopkgtest-cli trigger -package <name> [-version <ver>] [-arch <archs>] [-suite <suite>] [-trigger <pkg>]\n\n" +
		"Examples:\n" +
		"\tautopkgtest-cli check -package ovn\n" +
		"\tautopkgtest-cli check -package ovn -verbose\n" +
		"\tautopkgtest-cli trigger -package ovn -arch amd64,arm64 -suite noble\n")
}

func handleCheck(packageName string, verbose bool) {
	fmt.Printf("Checking autopkgtest results for package: %s\n", packageName)
	fmt.Println()

	s := scraper.NewScraper()
	results, err := s.FetchPackageResults(packageName)
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

func handleTrigger(packageName, version, suite, triggerPkg string, archs []string) {
	fmt.Println("Triggering autopkgtest (skeleton implementation)")
	fmt.Println()

	req := &trigger.TriggerRequest{
		Package:       packageName,
		Version:       version,
		Suite:         suite,
		Trigger:       triggerPkg,
		Architectures: archs,
	}

	fmt.Println("Request details:")
	fmt.Print(req.String())
	fmt.Println()

	t := trigger.NewTrigger()
	resp, err := t.TriggerTest(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error triggering test: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Success: %v\n", resp.Success)
	fmt.Printf("Message: %s\n", resp.Message)
	if resp.JobID != "" {
		fmt.Printf("Job ID: %s\n", resp.JobID)
	}

	fmt.Println()
	fmt.Println("Note: This is a skeleton implementation. The actual Ubuntu autopkgtest")
	fmt.Println("infrastructure requires specific authentication and API endpoints.")
	fmt.Println("See: https://wiki.ubuntu.com/ProposedMigration#autopkgtests")
}
