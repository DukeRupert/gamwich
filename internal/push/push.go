package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dukerupert/gamwich/internal/model"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// ErrExpired is returned when a push subscription is no longer valid (410 Gone).
var ErrExpired = errors.New("push subscription expired")

// Payload is the JSON sent to the push service.
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url,omitempty"`
	Tag   string `json:"tag,omitempty"`
}

// Config holds VAPID configuration.
type Config struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
}

// Service handles sending web push notifications.
type Service struct {
	publicKey  string
	privateKey string
}

// NewService creates a new push service with VAPID keys.
func NewService(publicKey, privateKey string) *Service {
	return &Service{
		publicKey:  publicKey,
		privateKey: privateKey,
	}
}

// VAPIDPublicKey returns the VAPID public key for client-side subscription.
func (s *Service) VAPIDPublicKey() string {
	return s.publicKey
}

// Send sends a push notification to a subscription.
func (s *Service) Send(sub *model.PushSubscription, payload Payload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := webpush.SendNotification(data, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dhKey,
			Auth:   sub.AuthKey,
		},
	}, &webpush.Options{
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		Subscriber:      "mailto:noreply@gamwich.app",
		TTL:             86400,
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		return ErrExpired
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("push service returned %d", resp.StatusCode)
	}

	return nil
}

// GenerateVAPIDKeys generates a new ECDSA P-256 key pair for VAPID.
func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ECDSA key: %w", err)
	}

	pubBytes := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	publicKey = base64.RawURLEncoding.EncodeToString(pubBytes)
	privateKey = base64.RawURLEncoding.EncodeToString(key.D.Bytes())

	return publicKey, privateKey, nil
}
