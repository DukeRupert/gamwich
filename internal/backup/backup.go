package backup

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
)

// s3Client is an interface for testability.
type s3Client interface {
	PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Config holds S3-compatible storage configuration.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

// Config holds backup manager configuration.
type Config struct {
	S3     S3Config
	DBPath string
}

// State represents the backup manager state.
type State string

const (
	StateIdle     State = "idle"
	StateRunning  State = "running"
	StateDisabled State = "disabled"
	StateError    State = "error"
)

// Status holds the current backup manager status.
type Status struct {
	State      State      `json:"state"`
	LastBackup *time.Time `json:"last_backup,omitempty"`
	Error      string     `json:"error,omitempty"`
	InProgress bool       `json:"in_progress"`
}

// StatusCallback is called whenever the backup state changes.
type StatusCallback func(Status)

// cachedCreds stores passphrase and salt for scheduled backups (memory only).
type cachedCreds struct {
	passphrase string
	salt       []byte
}

// Manager manages encrypted backups to S3-compatible storage.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	status   Status
	callback StatusCallback

	db            *sql.DB
	backupStore   *store.BackupStore
	settingsStore *store.SettingsStore
	client        s3Client

	cachedCreds map[int64]*cachedCreds // householdID -> cached credentials

	cancel context.CancelFunc
	done   chan struct{}
}

// NewManager creates a new backup manager.
func NewManager(cfg Config, db *sql.DB, bs *store.BackupStore, ss *store.SettingsStore, callback StatusCallback) *Manager {
	m := &Manager{
		cfg:           cfg,
		db:            db,
		backupStore:   bs,
		settingsStore: ss,
		callback:      callback,
		cachedCreds:   make(map[int64]*cachedCreds),
		status:        Status{State: StateDisabled},
	}

	if cfg.S3.Bucket != "" && cfg.S3.AccessKey != "" && cfg.S3.SecretKey != "" {
		m.client = newS3Client(cfg.S3)
		m.status.State = StateIdle
	}

	return m
}

func newS3Client(cfg S3Config) *s3.Client {
	opts := s3.Options{
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	}
	if cfg.Endpoint != "" {
		opts.BaseEndpoint = aws.String(cfg.Endpoint)
	}
	return s3.New(opts)
}

// UpdateS3Config hot-reloads the S3 configuration.
func (m *Manager) UpdateS3Config(s3cfg S3Config) {
	m.mu.Lock()
	m.cfg.S3 = s3cfg
	if s3cfg.Bucket != "" && s3cfg.AccessKey != "" && s3cfg.SecretKey != "" {
		m.client = newS3Client(s3cfg)
		m.status.State = StateIdle
	} else {
		m.client = nil
		m.status.State = StateDisabled
	}
	status := m.status
	m.mu.Unlock()
	if m.callback != nil {
		m.callback(status)
	}
}

// Start begins the scheduled backup loop.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.status.State == StateDisabled {
		m.mu.Unlock()
		return
	}
	ctx, m.cancel = context.WithCancel(ctx)
	m.done = make(chan struct{})
	m.mu.Unlock()

	go func() {
		defer close(m.done)
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkSchedule(ctx)
			}
		}
	}()
}

// Stop gracefully stops the backup manager.
func (m *Manager) Stop() {
	m.mu.RLock()
	cancel := m.cancel
	done := m.done
	m.mu.RUnlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Status returns the current backup status.
func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(s Status) {
	m.mu.Lock()
	m.status = s
	m.mu.Unlock()
	if m.callback != nil {
		m.callback(s)
	}
}

// CacheKey caches the passphrase and salt for scheduled backups.
func (m *Manager) CacheKey(householdID int64, passphrase string, salt []byte) {
	m.mu.Lock()
	m.cachedCreds[householdID] = &cachedCreds{passphrase: passphrase, salt: salt}
	m.mu.Unlock()
}

// HasCachedKey returns whether credentials are cached for the household.
func (m *Manager) HasCachedKey(householdID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.cachedCreds[householdID]
	return ok
}

func (m *Manager) checkSchedule(ctx context.Context) {
	now := time.Now().UTC()

	settings, err := m.settingsStore.GetBackupSettings()
	if err != nil {
		return
	}

	if settings["backup_enabled"] != "true" {
		return
	}

	hour, _ := strconv.Atoi(settings["backup_schedule_hour"])
	if now.Hour() != hour || now.Minute() != 0 {
		return
	}

	var householdID int64 = 1
	m.mu.RLock()
	creds, hasCached := m.cachedCreds[householdID]
	m.mu.RUnlock()

	if !hasCached {
		log.Printf("backup: skipping scheduled backup for household %d - no cached credentials", householdID)
		return
	}

	if _, err := m.runBackup(ctx, householdID, creds.passphrase, creds.salt); err != nil {
		log.Printf("backup: scheduled backup failed: %v", err)
	}

	retentionDays, _ := strconv.Atoi(settings["backup_retention_days"])
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if err := m.Cleanup(ctx, householdID, retentionDays); err != nil {
		log.Printf("backup: cleanup failed: %v", err)
	}
}

// RunNow runs a backup immediately with the provided passphrase.
func (m *Manager) RunNow(ctx context.Context, householdID int64, passphrase string) (int64, error) {
	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()
	if client == nil {
		return 0, fmt.Errorf("backup not configured: S3 credentials missing")
	}

	settings, err := m.settingsStore.GetBackupSettings()
	if err != nil {
		return 0, fmt.Errorf("get backup settings: %w", err)
	}

	saltHex := settings["backup_passphrase_salt"]
	if saltHex == "" {
		return 0, fmt.Errorf("backup passphrase not configured")
	}

	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return 0, fmt.Errorf("decode salt: %w", err)
	}

	return m.runBackup(ctx, householdID, passphrase, salt)
}

func (m *Manager) runBackup(ctx context.Context, householdID int64, passphrase string, salt []byte) (int64, error) {
	// Copy S3 client and bucket under lock
	m.mu.RLock()
	client := m.client
	bucket := m.cfg.S3.Bucket
	m.mu.RUnlock()

	if client == nil {
		return 0, fmt.Errorf("backup not configured: S3 credentials missing")
	}

	m.setStatus(Status{State: StateRunning, InProgress: true})

	timestamp := time.Now().UTC().Format("2006-01-02T150405Z")
	filename := fmt.Sprintf("backup-%s.db.enc", timestamp)
	s3Key := fmt.Sprintf("%d/%s", householdID, filename)

	record, err := m.backupStore.Create(householdID, filename, s3Key)
	if err != nil {
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("create backup record: %w", err)
	}

	m.backupStore.UpdateStatus(record.ID, model.BackupStatusUploading, "")

	tmpDir := os.TempDir()
	dbCopy := filepath.Join(tmpDir, fmt.Sprintf("gamwich-backup-%d.db", record.ID))
	encFile := filepath.Join(tmpDir, fmt.Sprintf("gamwich-backup-%d.db.enc", record.ID))
	defer os.Remove(dbCopy)
	defer os.Remove(encFile)

	// Checkpoint WAL and copy database
	if _, err := m.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		m.backupStore.UpdateStatus(record.ID, model.BackupStatusFailed, err.Error())
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("wal checkpoint: %w", err)
	}

	if err := copyFile(m.cfg.DBPath, dbCopy); err != nil {
		m.backupStore.UpdateStatus(record.ID, model.BackupStatusFailed, err.Error())
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("copy database: %w", err)
	}

	// Encrypt
	if err := EncryptFile(dbCopy, encFile, passphrase, salt); err != nil {
		m.backupStore.UpdateStatus(record.ID, model.BackupStatusFailed, err.Error())
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("encrypt: %w", err)
	}

	// Upload to S3
	encData, err := os.Open(encFile)
	if err != nil {
		m.backupStore.UpdateStatus(record.ID, model.BackupStatusFailed, err.Error())
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("open encrypted file: %w", err)
	}
	defer encData.Close()

	stat, _ := encData.Stat()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(s3Key),
		Body:          encData,
		ContentLength: aws.Int64(stat.Size()),
	})
	if err != nil {
		m.backupStore.UpdateStatus(record.ID, model.BackupStatusFailed, err.Error())
		m.setStatus(Status{State: StateError, Error: err.Error()})
		return 0, fmt.Errorf("upload to s3: %w", err)
	}

	m.backupStore.UpdateCompleted(record.ID, stat.Size())

	now := time.Now().UTC()
	m.setStatus(Status{State: StateIdle, LastBackup: &now})

	return record.ID, nil
}

// Restore downloads a backup from S3, decrypts it, validates it, replaces the DB file, and exits.
func (m *Manager) Restore(ctx context.Context, backupID, householdID int64, passphrase string) error {
	m.mu.RLock()
	client := m.client
	bucket := m.cfg.S3.Bucket
	m.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("backup not configured")
	}

	record, err := m.backupStore.GetByID(backupID, householdID)
	if err != nil {
		return fmt.Errorf("get backup: %w", err)
	}
	if record == nil {
		return fmt.Errorf("backup not found")
	}

	tmpDir := os.TempDir()
	encFile := filepath.Join(tmpDir, fmt.Sprintf("gamwich-restore-%d.db.enc", backupID))
	decFile := filepath.Join(tmpDir, fmt.Sprintf("gamwich-restore-%d.db", backupID))
	defer os.Remove(encFile)
	defer os.Remove(decFile)

	// Download from S3
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(record.S3Key),
	})
	if err != nil {
		return fmt.Errorf("download from s3: %w", err)
	}
	defer result.Body.Close()

	outFile, err := os.Create(encFile)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(outFile, result.Body); err != nil {
		outFile.Close()
		return fmt.Errorf("write downloaded file: %w", err)
	}
	outFile.Close()

	// Decrypt
	if err := DecryptFile(encFile, decFile, passphrase); err != nil {
		return fmt.Errorf("decrypt backup: %w", err)
	}

	// Validate SQLite integrity
	tmpDB, err := sql.Open("sqlite", decFile)
	if err != nil {
		return fmt.Errorf("open restored db: %w", err)
	}
	var integrity string
	if err := tmpDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		tmpDB.Close()
		return fmt.Errorf("integrity check: %w", err)
	}
	tmpDB.Close()
	if integrity != "ok" {
		return fmt.Errorf("integrity check failed: %s", integrity)
	}

	// Replace database file
	if err := copyFile(decFile, m.cfg.DBPath); err != nil {
		return fmt.Errorf("replace database: %w", err)
	}

	// Remove WAL and SHM files
	os.Remove(m.cfg.DBPath + "-wal")
	os.Remove(m.cfg.DBPath + "-shm")

	log.Printf("backup: restore complete, exiting for restart")
	os.Exit(0)
	return nil // unreachable
}

// Download streams an encrypted backup from S3.
func (m *Manager) Download(ctx context.Context, backupID, householdID int64) (io.ReadCloser, int64, error) {
	m.mu.RLock()
	client := m.client
	bucket := m.cfg.S3.Bucket
	m.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("backup not configured")
	}

	record, err := m.backupStore.GetByID(backupID, householdID)
	if err != nil {
		return nil, 0, fmt.Errorf("get backup: %w", err)
	}
	if record == nil {
		return nil, 0, fmt.Errorf("backup not found")
	}

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(record.S3Key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("download from s3: %w", err)
	}

	return result.Body, record.SizeBytes, nil
}

// Cleanup deletes backups older than the retention period.
func (m *Manager) Cleanup(ctx context.Context, householdID int64, retentionDays int) error {
	m.mu.RLock()
	client := m.client
	bucket := m.cfg.S3.Bucket
	m.mu.RUnlock()

	if client == nil {
		return nil
	}

	before := time.Now().UTC().AddDate(0, 0, -retentionDays)
	keys, err := m.backupStore.DeleteOlderThan(householdID, before)
	if err != nil {
		return fmt.Errorf("delete old backups: %w", err)
	}

	for _, key := range keys {
		if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}); err != nil {
			log.Printf("backup: failed to delete S3 object %s: %v", key, err)
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
