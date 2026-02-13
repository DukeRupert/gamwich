package email

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendAuthCodeLogin(t *testing.T) {
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
	transport := &rewriteTransport{base: http.DefaultTransport, target: server.URL}
	client.httpClient = &http.Client{Transport: transport}

	err := client.SendAuthCode("alice@example.com", "123456", "login", "")
	if err != nil {
		t.Fatalf("send auth code: %v", err)
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
	// Verify the body contains a clickable verify link
	expectedURL := "https://gamwich.test/auth/verify?token=123456"
	if !strings.Contains(received.TextBody, expectedURL) {
		t.Errorf("TextBody should contain verify URL, got: %s", received.TextBody)
	}
	if !strings.Contains(received.HtmlBody, expectedURL) {
		t.Errorf("HtmlBody should contain verify URL, got: %s", received.HtmlBody)
	}
	if !strings.Contains(received.HtmlBody, "Sign in to Gamwich") {
		t.Errorf("HtmlBody should contain link text, got: %s", received.HtmlBody)
	}
}

func TestSendAuthCodeInvite(t *testing.T) {
	var received postmarkEmail

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"MessageID": "test-id"}`))
	}))
	defer server.Close()

	client := NewClient("test-token", "noreply@example.com", "https://gamwich.test")
	client.httpClient = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: server.URL}}

	err := client.SendAuthCode("bob@example.com", "654321", "invite", "Smith Family")
	if err != nil {
		t.Fatalf("send auth code: %v", err)
	}

	if received.Subject != "You've been invited to Smith Family on Gamwich" {
		t.Errorf("Subject = %q, want invite subject", received.Subject)
	}
	// Invite emails should contain both a navigation URL and the code
	if !strings.Contains(received.TextBody, "654321") {
		t.Errorf("TextBody should contain code, got: %s", received.TextBody)
	}
	if !strings.Contains(received.TextBody, "https://gamwich.test/invite/accept?email=bob%40example.com") {
		t.Errorf("TextBody should contain navigation URL, got: %s", received.TextBody)
	}
	if !strings.Contains(received.HtmlBody, "654321") {
		t.Errorf("HtmlBody should contain code, got: %s", received.HtmlBody)
	}
	if !strings.Contains(received.HtmlBody, "https://gamwich.test/invite/accept?email=bob%40example.com") {
		t.Errorf("HtmlBody should contain navigation URL, got: %s", received.HtmlBody)
	}
}

func TestSendAuthCodeNotConfigured(t *testing.T) {
	client := NewClient("", "noreply@example.com", "https://gamwich.test")

	err := client.SendAuthCode("alice@example.com", "123456", "login", "")
	if err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestSendAuthCodeAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := NewClient("test-token", "noreply@example.com", "https://gamwich.test")
	client.httpClient = &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: server.URL}}

	err := client.SendAuthCode("alice@example.com", "123456", "login", "")
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
