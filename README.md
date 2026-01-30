# Autopkgtest Automation CLI

A command-line tool for interacting with Ubuntu's autopkgtest infrastructure. This tool allows you to check test results and trigger tests for Ubuntu packages.

## Installation

### Prerequisites

- Go 1.21 or higher
- Make (optional)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/canonical/autopkgtest-automation.git
cd autopkgtest-automation

# Build the binary
make build

# Or use go compiler directly
go build -o build/autopkgtest-cli ./cmd/autopkgtest-cli
```

### Install System-wide

```bash
make install
```

This will install the binary to `/usr/local/bin/autopkgtest-cli`.

### Running Tests

Run all tests:

```bash
make test
```

Run tests with coverage:

```bash
make test-coverage
```

This will generate a `coverage.html` file.

### Available Make Targets

- `make build`: Build the application
- `make test`: Run all tests
- `make test-coverage`: Run tests with coverage report
- `make clean`: Clean build artifacts
- `make install`: Install to system
- `make uninstall`: Remove from system
- `make run-check`: Run check command with example
- `make run-trigger`: Run trigger command with example
- `make deps`: Download and tidy dependencies
- `make fmt`: Format Go code
- `make vet`: Run go vet
- `make check-all`: Run all checks (format, vet, test)
- `make help`: Show help with all targets

## Usage

### Check Package Test Results

Check autopkgtest results for a package:

```bash
autopkgtest-cli check -package ovn
```

Show all test results (not just errors):

```bash
autopkgtest-cli check -package ovn -verbose
```

Filter results by release (Ubuntu version):

```bash
autopkgtest-cli check -package ovn -release noble
```

Filter results by architecture:

```bash
autopkgtest-cli check -package ovn -arch amd64
```

Combine filters for specific release/architecture:

```bash
autopkgtest-cli check -package ovn -release noble -arch amd64 -verbose
```

### Generate Trigger URLs

Generate autopkgtest trigger URL(s) for manual browser triggering:

```bash
# Basic trigger URL for all architectures
autopkgtest-cli generate-trigger-link -package ovn -suite noble

# Generate URLs for specific architectures
autopkgtest-cli generate-trigger-link -package ovn -suite noble -arch amd64,arm64

# With specific version
autopkgtest-cli generate-trigger-link -package ovn -suite noble -version 24.03.1-1

# Test against a PPA
autopkgtest-cli generate-trigger-link -package myapp -suite jammy -ppa myuser/testing-ppa
```

### Trigger Tests Automatically

Trigger autopkgtests automatically with authentication:

```bash
# Basic trigger (requires cookies)
autopkgtest-cli trigger -package ovn -suite noble -credentials ~/.autopkgtest-cookies

# Trigger for specific architectures
autopkgtest-cli trigger -package ovn -suite noble -arch amd64,arm64 -credentials ~/.autopkgtest-cookies

# Trigger and wait for completion
autopkgtest-cli trigger -package ovn -suite noble --wait --timeout 1h -credentials ~/.autopkgtest-cookies

# Test against a PPA
autopkgtest-cli trigger -package myapp -suite jammy -ppa myuser/testing-ppa -credentials ~/.autopkgtest-cookies
```

**Authentication Setup:**

The `trigger` command requires Launchpad authentication.

**Monitoring Options:**
- `--wait`: Wait for test completion before exiting (streams logs in real-time)
- `--timeout`: Maximum time to wait (default: 2h)
- `--poll-interval`: How often to check status and refresh logs (default: 30s)

When using `--wait`, the CLI will stream test logs in real-time as they're generated, so you can monitor test progress without opening the browser.

### Available Commands

- `check`: Check autopkgtest results for a package
- `generate-trigger-link`: Generate autopkgtest trigger URLs for manual browser triggering
- `trigger`: Trigger autopkgtests automatically with authentication
- `version`: Show version information
- `help`: Show help message

### Command Options

#### Check Command

```
autopkgtest-cli check [flags]

Flags:
  -package string    Package name to check (required)
  -verbose           Show all test results, not just errors
  -release string    Filter by specific Ubuntu release (optional, e.g., noble, jammy)
  -arch string       Filter by specific architecture (optional, e.g., amd64, arm64)
```

**Filtering Examples:**
- Check only noble results: `-release noble`
- Check only amd64 results: `-arch amd64`
- Check noble/amd64 combination: `-release noble -arch amd64`

#### Generate-Trigger-Link Command

```
autopkgtest-cli generate-trigger-link [flags]

Flags:
  -package string      Package name (required)
  -suite string        Ubuntu release/suite (required, e.g., noble, mantic, jammy)
  -arch string         Comma-separated list of architectures (optional, e.g., amd64,arm64)
  -version string      Package version (optional)
  -trigger string      Custom trigger string (optional, overrides package/version)
  -ppa string          PPA to test against (optional, format: user/ppa-name)
  -all-proposed        Install all packages from proposed pocket (optional)
```

#### Trigger Command

```
autopkgtest-cli trigger [flags]

Flags:
  -package string         Package name (required)
  -suite string           Ubuntu release/suite (required, e.g., noble, mantic, jammy)
  -arch string            Comma-separated list of architectures (optional, e.g., amd64,arm64)
  -version string         Package version (optional)
  -trigger string         Custom trigger string (optional, overrides package/version)
  -ppa string             PPA to test against (optional, format: user/ppa-name)
  -all-proposed           Install all packages from proposed pocket (optional)
  -credentials string     Path to cookie file with Launchpad session (required for auth)
  -wait                   Wait for test completion
  -timeout duration       Maximum time to wait for completion (default: 2h)
  -poll-interval duration How often to check test status (default: 30s)
```

## How It Works

### Web Scraping

The tool fetches HTML pages from `https://autopkgtest.ubuntu.com/packages/<package-name>` and parses the results matrix table. The autopkgtest page displays results in a matrix format:

- **Columns**: Ubuntu releases (focal, jammy, noble, questing, resolute, etc.)
- **Rows**: Architectures (amd64, arm64, armhf, i386, ppc64el, riscv64, s390x)
- **Cells**: Test status for each release/architecture combination

The scraper extracts:

- Test status (pass, fail, neutral, regression, tmpfail)
- Ubuntu release name
- Architecture
- Links to detailed test results pages

### Error Detection

Tests are classified as errors if their status is not "pass" or "neutral". This includes:
- `fail`: Test failed
- `regression`: New failure compared to previous version
- `tmpfail`: Temporary failure (infrastructure issues)

The tool automatically filters and reports these errors with detailed information and links to full logs.

### Trigger URL Generation

The trigger functionality generates proper autopkgtest request URLs following the official Ubuntu autopkgtest infrastructure format. The URLs are based on the pattern:

```
https://autopkgtest.ubuntu.com/request.cgi?release=RELEASE&arch=ARCH&package=PACKAGE&trigger=TRIGGER
```

**How it works:**

1. **User Authentication**: You must be logged into Launchpad in your browser with upload rights for the package
2. **URL Generation**: The tool generates properly formatted URLs with all required parameters
3. **Manual Triggering**: Open the generated URL(s) in your browser to submit the test request
4. **Test Execution**: The autopkgtest infrastructure validates your permissions and queues the test

**Parameters:**
- `release`: Ubuntu release codename (noble, jammy, etc.)
- `package`: Source package name to test
- `trigger`: Package/version that triggered the test (defaults to migration-reference/0)
- `arch`: Architecture (optional, if omitted tests all architectures)
- `ppa`: PPA identifier for testing against a PPA (optional)
- `all-proposed`: Flag to use all packages from proposed pocket (optional)

This is the official and recommended way to trigger autopkgtests. See [Ubuntu's autopkgtest documentation](https://wiki.ubuntu.com/ProposedMigration#autopkgtests) for more details.

## Examples

### Check a package for errors

```bash
$ autopkgtest-cli check -package ovn
Checking autopkgtest results for package: ovn

Found 14 errors for package: ovn

Error 1:
  Status: fail
  Release: noble
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/questing/amd64

Error 2:
  Status: fail
  Release: questing
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/resolute/amd64

Error 3:
  Status: fail
  Release: noble
  Architecture: arm64
  Details: https://autopkgtest.ubuntu.com/ovn/questing/arm64

...
```

### Verbose output

```bash
$ autopkgtest-cli check -package ovn -verbose
Checking autopkgtest results for package: ovn

Total tests found: 26

All test results:

Test 1:
  Status: neutral
  Release: focal
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/focal/amd64

Test 2:
  Status: pass
  Release: jammy
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/jammy/amd64

Test 3:
  Status: fail
  Release: noble
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/noble/amd64

...

Found 14 errors for package: ovn

Error 1:
  Status: fail
  Release: noble
  Architecture: amd64
  Details: https://autopkgtest.ubuntu.com/ovn/questing/amd64

...
```

## Related Links

- [Ubuntu Autopkgtest](https://autopkgtest.ubuntu.com/)
- [Autopkgtest Documentation](https://salsa.debian.org/ci-team/autopkgtest)

## License

autopkgtests-automation is free software, distributed under the AGPLv3 license (GNU Affero General Public License version 3.0).
Refer to the [LICENSE](./LICENSE) file (the actual license) for more information.