package backup

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// mockS3Client implements s3Client for testing.
type mockS3Client struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
	getErr  error
	delErr  error
}

func newMockS3() *mockS3Client {
	return &mockS3Client{objects: make(map[string][]byte)}
}

func (m *mockS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := io.ReadAll(input.Body)
	m.objects[*input.Key] = data
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[*input.Key]
	if !ok {
		return nil, &s3NotFound{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(string(data))),
	}, nil
}

func (m *mockS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if m.delErr != nil {
		return nil, m.delErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, *input.Key)
	return &s3.DeleteObjectOutput{}, nil
}

type s3NotFound struct{}

func (e *s3NotFound) Error() string { return "NoSuchKey" }

func TestManagerStateLifecycle(t *testing.T) {
	// Without S3 config -> disabled
	m := NewManager(Config{}, nil, nil, nil, nil)
	if m.Status().State != StateDisabled {
		t.Errorf("state = %q, want %q", m.Status().State, StateDisabled)
	}

	// With S3 config -> idle
	m2 := NewManager(Config{
		S3: S3Config{Bucket: "test", AccessKey: "key", SecretKey: "secret"},
	}, nil, nil, nil, nil)
	if m2.Status().State != StateIdle {
		t.Errorf("state = %q, want %q", m2.Status().State, StateIdle)
	}
}

func TestManagerStatusCallback(t *testing.T) {
	var received []Status
	var mu sync.Mutex
	cb := func(s Status) {
		mu.Lock()
		received = append(received, s)
		mu.Unlock()
	}

	m := NewManager(Config{
		S3: S3Config{Bucket: "test", AccessKey: "key", SecretKey: "secret"},
	}, nil, nil, nil, cb)

	m.setStatus(Status{State: StateRunning, InProgress: true})
	m.setStatus(Status{State: StateIdle})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("received %d callbacks, want 2", len(received))
	}
	if received[0].State != StateRunning {
		t.Errorf("first callback state = %q, want %q", received[0].State, StateRunning)
	}
	if received[1].State != StateIdle {
		t.Errorf("second callback state = %q, want %q", received[1].State, StateIdle)
	}
}

func TestManagerStopSafety(t *testing.T) {
	m := NewManager(Config{
		S3: S3Config{Bucket: "test", AccessKey: "key", SecretKey: "secret"},
	}, nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()
	m.Stop()

	// Double stop should not panic
	m.Stop()
}

func TestManagerCachedKey(t *testing.T) {
	m := NewManager(Config{}, nil, nil, nil, nil)

	if m.HasCachedKey(1) {
		t.Error("expected no cached key")
	}

	m.CacheKey(1, "passphrase", []byte("salt1234salt1234"))

	if !m.HasCachedKey(1) {
		t.Error("expected cached key")
	}
	if m.HasCachedKey(2) {
		t.Error("expected no cached key for different household")
	}
}

func TestUpdateS3Config(t *testing.T) {
	var received []Status
	var mu sync.Mutex
	cb := func(s Status) {
		mu.Lock()
		received = append(received, s)
		mu.Unlock()
	}

	// Start disabled
	m := NewManager(Config{}, nil, nil, nil, cb)
	if m.Status().State != StateDisabled {
		t.Fatalf("initial state = %q, want %q", m.Status().State, StateDisabled)
	}

	// Set valid config -> transitions to idle
	m.UpdateS3Config(S3Config{Bucket: "test", AccessKey: "key", SecretKey: "secret", Region: "us-east-1"})
	if m.Status().State != StateIdle {
		t.Errorf("state after set = %q, want %q", m.Status().State, StateIdle)
	}

	// Clear config -> transitions back to disabled
	m.UpdateS3Config(S3Config{})
	if m.Status().State != StateDisabled {
		t.Errorf("state after clear = %q, want %q", m.Status().State, StateDisabled)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("received %d callbacks, want 2", len(received))
	}
	if received[0].State != StateIdle {
		t.Errorf("first callback state = %q, want %q", received[0].State, StateIdle)
	}
	if received[1].State != StateDisabled {
		t.Errorf("second callback state = %q, want %q", received[1].State, StateDisabled)
	}
}

func TestManagerDisabledNoStart(t *testing.T) {
	m := NewManager(Config{}, nil, nil, nil, nil)

	ctx := context.Background()
	m.Start(ctx) // should be a no-op for disabled state

	// Stop should not block
	m.Stop()
}
