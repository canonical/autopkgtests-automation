package autopkgtestclient

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.httpClient == nil {
		t.Error("Expected http client to be initialized")
	}

	if client.baseURL != "https://autopkgtest.ubuntu.com" {
		t.Errorf("Expected baseURL to be https://autopkgtest.ubuntu.com, got %s", client.baseURL)
	}
}

func TestTriggerTest_Success(t *testing.T) {
	// Mock server that returns successful test submission
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `Logout testuser

Test request submitted.

Result history
    https://autopkgtest.ubuntu.com/packages/ovn/noble/amd64
Result url
    https://autopkgtest.ubuntu.com/run/ae232d9f-08bd-4e36-90b7-7e3811776a64
UUID
    ae232d9f-08bd-4e36-90b7-7e3811776a64
arch
    amd64
package
    ovn
release
    noble
requester
    testuser
triggers
    ['migration-reference/0']`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	result, err := client.TriggerTest(server.URL)
	if err != nil {
		t.Fatalf("TriggerTest() failed: %v", err)
	}

	if result.UUID != "ae232d9f-08bd-4e36-90b7-7e3811776a64" {
		t.Errorf("Expected UUID ae232d9f-08bd-4e36-90b7-7e3811776a64, got %s", result.UUID)
	}

	if result.Package != "ovn" {
		t.Errorf("Expected package ovn, got %s", result.Package)
	}

	if result.Release != "noble" {
		t.Errorf("Expected release noble, got %s", result.Release)
	}

	if result.Arch != "amd64" {
		t.Errorf("Expected arch amd64, got %s", result.Arch)
	}

	if result.Requester != "testuser" {
		t.Errorf("Expected requester testuser, got %s", result.Requester)
	}

	if !strings.Contains(result.ResultURL, "ae232d9f-08bd-4e36-90b7-7e3811776a64") {
		t.Errorf("Expected ResultURL to contain UUID, got %s", result.ResultURL)
	}
}

func TestTriggerTest_AlreadyRunning(t *testing.T) {
	// Mock server that returns "test already running" error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `Logout testuser

You submitted an invalid request:

Test already running:

release: noble

pkg: ovn

arch: amd64

triggers: migration-reference/0`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	_, err = client.TriggerTest(server.URL)
	if err == nil {
		t.Fatal("Expected error for already running test")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("Expected 'already running' error, got: %v", err)
	}
}

func TestTriggerTest_AuthRequired(t *testing.T) {
	// Mock server that returns login page content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<html><body>Please login to continue</body></html>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Use the login URL pattern
	loginURL := server.URL + "/login?next=" + server.URL

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	_, err = client.TriggerTest(loginURL)
	if err == nil {
		t.Fatal("Expected authentication error")
	}

	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("Expected authentication required error, got: %v", err)
	}
}

func TestTriggerTest_InvalidRequest_WithLogout(t *testing.T) {
	// Mock server that returns invalid request error with Logout link (user is authenticated)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<head>
<meta charset="utf-8">
<title>Autopkgtest Test Request</title>
</head>
<body>

<p><a href="/logout">Logout matperin</a></p>
<p>You submitted an invalid request: openssl/3.5.4-1ubuntu1 is not published in noble</p>

</body>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	_, err = client.TriggerTest(server.URL)
	if err == nil {
		t.Fatal("Expected invalid request error")
	}

	// Should be invalid request, not authentication error
	if !strings.Contains(err.Error(), "invalid request") {
		t.Errorf("Expected 'invalid request' error, got: %v", err)
	}

	if strings.Contains(err.Error(), "authentication") {
		t.Errorf("Should not be authentication error, got: %v", err)
	}

	// Should contain the specific error message
	if !strings.Contains(err.Error(), "is not published in noble") {
		t.Errorf("Expected error message to contain 'is not published in noble', got: %v", err)
	}
}

func TestGetTestStatus_Running(t *testing.T) {
	// Mock server that returns running status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `Test In progress...`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.GetTestStatus("test-uuid-123")
	if err != nil {
		t.Fatalf("GetTestStatus() failed: %v", err)
	}

	if status.Status != "running" {
		t.Errorf("Expected status 'running', got %s", status.Status)
	}

	if status.UUID != "test-uuid-123" {
		t.Errorf("Expected UUID test-uuid-123, got %s", status.UUID)
	}
}

func TestGetTestStatus_Pass(t *testing.T) {
	// Mock server that returns passing status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `| Result | ✔ pass |
| Duration | 15m 32s |`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.GetTestStatus("test-uuid-123")
	if err != nil {
		t.Fatalf("GetTestStatus() failed: %v", err)
	}

	if status.Status != "pass" {
		t.Errorf("Expected status 'pass', got %s", status.Status)
	}

	if status.Duration != "15m 32s" {
		t.Errorf("Expected duration '15m 32s', got %s", status.Duration)
	}
}

func TestGetTestStatus_Fail(t *testing.T) {
	// Mock server that returns failing status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `| Result | ✖ fail |`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.GetTestStatus("test-uuid-123")
	if err != nil {
		t.Fatalf("GetTestStatus() failed: %v", err)
	}

	if status.Status != "fail" {
		t.Errorf("Expected status 'fail', got %s", status.Status)
	}
}

func TestGetTestStatus_TmpFail(t *testing.T) {
	// Mock server that returns tmpfail status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `| Result | ⚠ tmpfail |`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.GetTestStatus("test-uuid-123")
	if err != nil {
		t.Fatalf("GetTestStatus() failed: %v", err)
	}

	if status.Status != "tmpfail" {
		t.Errorf("Expected status 'tmpfail', got %s", status.Status)
	}
}

func TestGetTestStatus_Neutral(t *testing.T) {
	// Mock server that returns neutral status
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `| Result | 😐neutral |`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.GetTestStatus("test-uuid-123")
	if err != nil {
		t.Fatalf("GetTestStatus() failed: %v", err)
	}

	if status.Status != "neutral" {
		t.Errorf("Expected status 'neutral', got %s", status.Status)
	}
}

func TestWaitForCompletion(t *testing.T) {
	// Track number of requests to simulate test progression
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var response string
		if requestCount <= 2 {
			response = `Test In progress...`
		} else {
			response = `| Result | ✔ pass |
| Duration | 10m 5s |`
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	status, err := client.WaitForCompletion("testpkg", "test-uuid", 100*time.Millisecond, 1*time.Second)
	if err != nil {
		t.Fatalf("WaitForCompletion() failed: %v", err)
	}

	if status.Status != "pass" {
		t.Errorf("Expected final status 'pass', got %s", status.Status)
	}

	if requestCount < 2 {
		t.Errorf("Expected multiple polling requests, got %d", requestCount)
	}
}

func TestWaitForCompletion_Timeout(t *testing.T) {
	// Server always returns "running"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `Test In progress...`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	_, err = client.WaitForCompletion("testpkg", "test-uuid", 50*time.Millisecond, 200*time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestWithCookies(t *testing.T) {
	testCookie := &http.Cookie{
		Name:  "session",
		Value: "test-session-id",
	}

	client, err := NewClient(WithCookies([]*http.Cookie{testCookie}))
	if err != nil {
		t.Fatalf("NewClient() with cookies failed: %v", err)
	}

	cookies := client.GetCookies()
	if len(cookies) == 0 {
		t.Error("Expected cookies to be set")
	}
}

func TestFindRunningTest(t *testing.T) {
	// Mock server that returns package page with running test
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/packages/") {
			// Simulate the running tests section with actual HTML format
			response := `<html>
				<body>
					<h3>Running tests</h3>
					<table>
						<tr>
							<th>Release:</th>
							<td>noble</td>
						</tr>
						<tr>
							<th>Architecture:</th>
							<td>amd64</td>
						</tr>
						<tr>
							<th>UUID:</th>
							<td>12345678-1234-1234-1234-123456789abc</td>
						</tr>
						<tr>
							<th>Running for:</th>
							<td>1h 30m</td>
						</tr>
					</table>
				</body>
			</html>`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		} else if strings.Contains(r.URL.Path, "/run/") {
			response := `Test In progress...`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	uuid, err := client.FindRunningTest("testpkg", "noble", "amd64")
	if err != nil {
		t.Fatalf("FindRunningTest() failed: %v", err)
	}

	if uuid != "12345678-1234-1234-1234-123456789abc" {
		t.Errorf("Expected UUID 12345678-1234-1234-1234-123456789abc, got %s", uuid)
	}
}

func TestFindRunningTest_NotFound(t *testing.T) {
	// Mock server that returns no running tests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `<html><body>No running tests</body></html>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client.baseURL = server.URL

	_, err = client.FindRunningTest("testpkg", "noble", "amd64")
	if err == nil {
		t.Fatal("Expected error when no running test found")
	}
}
