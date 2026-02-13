package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type Client struct {
	mu          sync.RWMutex
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverToken != ""
}

type postmarkEmail struct {
	From     string `json:"From"`
	To       string `json:"To"`
	Subject  string `json:"Subject"`
	HtmlBody string `json:"HtmlBody"`
	TextBody string `json:"TextBody"`
}

// SendAuthCode sends an authentication code email for login, registration, or invitation.
func (c *Client) SendAuthCode(toEmail, code, purpose, householdName string) error {
	// Copy config under lock
	c.mu.RLock()
	serverToken := c.serverToken
	fromEmail := c.fromEmail
	baseURL := c.baseURL
	httpClient := c.httpClient
	c.mu.RUnlock()

	if serverToken == "" {
		return fmt.Errorf("email client not configured: missing server token")
	}

	var subject, textBody, htmlBody string
	switch purpose {
	case "login":
		subject = "Sign in to Gamwich"
		verifyURL := fmt.Sprintf("%s/auth/verify?token=%s", baseURL, url.QueryEscape(code))
		textBody = fmt.Sprintf("Sign in to Gamwich:\n\n%s\n\nThis link expires in 15 minutes.", verifyURL)
		htmlBody = fmt.Sprintf(
			`<p>Click the link below to sign in to Gamwich:</p><p><a href="%s" style="display:inline-block;padding:12px 24px;background-color:#6366f1;color:#fff;text-decoration:none;border-radius:8px;font-weight:bold">Sign in to Gamwich</a></p><p style="font-size:12px;color:#666">This link expires in 15 minutes.</p>`,
			verifyURL,
		)
	case "register":
		subject = "Welcome to Gamwich"
		textBody = fmt.Sprintf("Your registration code is: %s\n\nEnter this code to complete your registration. It expires in 15 minutes.", code)
		htmlBody = fmt.Sprintf(
			`<p>Your registration code is:</p><p style="font-size:32px;font-weight:bold;letter-spacing:4px">%s</p><p>Enter this code to complete your registration. It expires in 15 minutes.</p>`,
			code,
		)
	case "invite":
		subject = fmt.Sprintf("You've been invited to %s on Gamwich", householdName)
		inviteURL := fmt.Sprintf("%s/invite/accept?email=%s", baseURL, url.QueryEscape(toEmail))
		textBody = fmt.Sprintf("You've been invited to %s on Gamwich!\n\nVisit: %s\n\nYour code is: %s\n\nThis code expires in 15 minutes.", householdName, inviteURL, code)
		htmlBody = fmt.Sprintf(
			`<p>You've been invited to <strong>%s</strong> on Gamwich!</p><p><a href="%s">Click here to accept your invitation</a></p><p>Your code is:</p><p style="font-size:32px;font-weight:bold;letter-spacing:4px">%s</p><p>This code expires in 15 minutes.</p>`,
			householdName, inviteURL, code,
		)
	default:
		subject = "Your Gamwich code"
		textBody = fmt.Sprintf("Your code is: %s\n\nThis code expires in 15 minutes.", code)
		htmlBody = fmt.Sprintf(
			`<p>Your code is:</p><p style="font-size:32px;font-weight:bold;letter-spacing:4px">%s</p><p>This code expires in 15 minutes.</p>`,
			code,
		)
	}

	payload := postmarkEmail{
		From:     fromEmail,
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
	req.Header.Set("X-Postmark-Server-Token", serverToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("postmark API error: status %d", resp.StatusCode)
	}

	return nil
}
