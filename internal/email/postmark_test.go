package email

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMagicLinkLogin(t *testing.T) {
	var received postmarkEmail
	var gotToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Postmark-Server-Token")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"MessageID": "test-id"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "noreply@example.com", "https://gamwich.test", WithHTTPClient(server.Client()))
	// Override the postmark URL by using the test server URL
	// We need to adjust: the client currently hardcodes the API URL.
	// For testing, we'll create a custom HTTP client that redirects to our test server.
	transport := &rewriteTransport{base: http.DefaultTransport, target: server.URL}
	client.httpClient = &http.Client{Transport: transport}

	err := client.SendMagicLink("alice@example.com", "abc123", "login", "")
	if err != nil {
		t.Fatalf("send magic link: %v", err)
	}

	if gotToken != "test-token" {
		t.Errorf("server token = %q, want %q", gotToken, "test-token")
	}
	if received.To != "alice@example.com" {
		t.Errorf("To = %q, want %q", received.To, "alice@example.com")
	}
	if received.From != "noreply@example.com" {
		t.Errorf("From = %q, want %q", received.From, "noreply@example.com")
	}
	if received.Subject != "Sign in to Gamwich" {
		t.Errorf("Subject = %q, want %q", received.Subject, "Sign in to Gamwich")
	}
}

func TestSendMagicLinkInvite(t *testing.T) {
	var received postmarkEmail

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"MessageID": "test-id"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "noreply@example.com", "https://gamwich.test")
	client.httpClient = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: server.URL}}

	err := client.SendMagicLink("bob@example.com", "xyz789", "invite", "Smith Family")
	if err != nil {
		t.Fatalf("send magic link: %v", err)
	}

	if received.Subject != "You've been invited to Smith Family on Gamwich" {
		t.Errorf("Subject = %q, want invite subject", received.Subject)
	}
}

func TestSendMagicLinkNotConfigured(t *testing.T) {
	client := NewClient("", "noreply@example.com", "https://gamwich.test")

	err := client.SendMagicLink("alice@example.com", "abc123", "login", "")
	if err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestSendMagicLinkAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := NewClient("test-token", "noreply@example.com", "https://gamwich.test")
	client.httpClient = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: server.URL}}

	err := client.SendMagicLink("alice@example.com", "abc123", "login", "")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

func TestConfigured(t *testing.T) {
	c1 := NewClient("token", "from@test.com", "https://test.com")
	if !c1.Configured() {
		t.Error("expected Configured() = true")
	}

	c2 := NewClient("", "from@test.com", "https://test.com")
	if c2.Configured() {
		t.Error("expected Configured() = false")
	}
}

func TestUpdateConfig(t *testing.T) {
	client := NewClient("", "", "")
	if client.Configured() {
		t.Error("expected Configured() = false initially")
	}

	client.UpdateConfig("new-token", "new@example.com", "https://new.example.com")
	if !client.Configured() {
		t.Error("expected Configured() = true after UpdateConfig")
	}

	// Verify updated fields are used
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Postmark-Server-Token")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"MessageID": "test-id"}`))
	}))
	defer server.Close()

	client.httpClient = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: server.URL}}
	err := client.SendMagicLink("alice@example.com", "tok123", "login", "")
	if err != nil {
		t.Fatalf("send after update: %v", err)
	}
	if gotToken != "new-token" {
		t.Errorf("server token = %q, want %q", gotToken, "new-token")
	}

	// Clear config
	client.UpdateConfig("", "", "")
	if client.Configured() {
		t.Error("expected Configured() = false after clearing")
	}
}

// rewriteTransport redirects all requests to a test server URL.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.target[len("http://"):]
	return t.base.RoundTrip(req)
}
