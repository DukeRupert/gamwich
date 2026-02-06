package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	serverToken string
	fromEmail   string
	baseURL     string
	httpClient  *http.Client
}

type Option func(*Client)

func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		cl.httpClient = c
	}
}

func NewClient(serverToken, fromEmail, baseURL string, opts ...Option) *Client {
	c := &Client{
		serverToken: serverToken,
		fromEmail:   fromEmail,
		baseURL:     baseURL,
		httpClient:  http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Configured returns true if the server token is set.
func (c *Client) Configured() bool {
	return c.serverToken != ""
}

type postmarkEmail struct {
	From     string `json:"From"`
	To       string `json:"To"`
	Subject  string `json:"Subject"`
	HtmlBody string `json:"HtmlBody"`
	TextBody string `json:"TextBody"`
}

// SendMagicLink sends a magic link email for login, registration, or invitation.
func (c *Client) SendMagicLink(toEmail, token, purpose, householdName string) error {
	if !c.Configured() {
		return fmt.Errorf("email client not configured: missing server token")
	}

	var subject, action string
	switch purpose {
	case "login":
		subject = "Sign in to Gamwich"
		action = "sign in"
	case "register":
		subject = "Welcome to Gamwich"
		action = "complete your registration"
	case "invite":
		subject = fmt.Sprintf("You've been invited to %s on Gamwich", householdName)
		action = "accept your invitation"
	default:
		subject = "Your Gamwich link"
		action = "continue"
	}

	link := fmt.Sprintf("%s/auth/verify?token=%s", c.baseURL, token)
	textBody := fmt.Sprintf("Click the link below to %s:\n\n%s\n\nThis link expires in 15 minutes.", action, link)
	htmlBody := fmt.Sprintf(
		`<p>Click the link below to %s:</p><p><a href="%s">%s</a></p><p>This link expires in 15 minutes.</p>`,
		action, link, action,
	)

	payload := postmarkEmail{
		From:     c.fromEmail,
		To:       toEmail,
		Subject:  subject,
		HtmlBody: htmlBody,
		TextBody: textBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.postmarkapp.com/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", c.serverToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("postmark API error: status %d", resp.StatusCode)
	}

	return nil
}
