package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
)

type BackupStore struct {
	db *sql.DB
}

func NewBackupStore(db *sql.DB) *BackupStore {
	return &BackupStore{db: db}
}

func (s *BackupStore) Create(householdID int64, filename, s3Key string) (*model.Backup, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(
		`INSERT INTO backups (household_id, filename, s3_key, status, started_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		householdID, filename, s3Key, model.BackupStatusPending, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create backup: %w", err)
	}
	id, _ := result.LastInsertId()
	return &model.Backup{
		ID:          id,
		HouseholdID: householdID,
		Filename:    filename,
		S3Key:       s3Key,
		Status:      model.BackupStatusPending,
		StartedAt:   &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *BackupStore) GetByID(id, householdID int64) (*model.Backup, error) {
	b := &model.Backup{}
	var errMsg sql.NullString
	var startedAt, completedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, household_id, filename, s3_key, size_bytes, status, error_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE id = ? AND household_id = ?`, id, householdID,
	).Scan(&b.ID, &b.HouseholdID, &b.Filename, &b.S3Key, &b.SizeBytes, &b.Status, &errMsg, &startedAt, &completedAt, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get backup %d: %w", id, err)
	}
	b.ErrorMessage = errMsg.String
	if startedAt.Valid {
		b.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		b.CompletedAt = &completedAt.Time
	}
	return b, nil
}

func (s *BackupStore) List(householdID int64, limit int) ([]model.Backup, error) {
	rows, err := s.db.Query(
		`SELECT id, household_id, filename, s3_key, size_bytes, status, error_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE household_id = ? ORDER BY created_at DESC LIMIT ?`, householdID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		var errMsg sql.NullString
		var startedAt, completedAt sql.NullTime
		if err := rows.Scan(&b.ID, &b.HouseholdID, &b.Filename, &b.S3Key, &b.SizeBytes, &b.Status, &errMsg, &startedAt, &completedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan backup: %w", err)
		}
		b.ErrorMessage = errMsg.String
		if startedAt.Valid {
			b.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			b.CompletedAt = &completedAt.Time
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

func (s *BackupStore) UpdateStatus(id int64, status model.BackupStatus, errorMsg string) error {
	var errPtr *string
	if errorMsg != "" {
		errPtr = &errorMsg
	}
	_, err := s.db.Exec(
		`UPDATE backups SET status = ?, error_message = ? WHERE id = ?`,
		status, errPtr, id,
	)
	if err != nil {
		return fmt.Errorf("update backup status: %w", err)
	}
	return nil
}

func (s *BackupStore) UpdateCompleted(id, sizeBytes int64) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE backups SET status = ?, size_bytes = ?, completed_at = ? WHERE id = ?`,
		model.BackupStatusCompleted, sizeBytes, now, id,
	)
	if err != nil {
		return fmt.Errorf("update backup completed: %w", err)
	}
	return nil
}

// DeleteOlderThan deletes backups older than the given time and returns the S3 keys of deleted backups.
func (s *BackupStore) DeleteOlderThan(householdID int64, before time.Time) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT s3_key FROM backups WHERE household_id = ? AND created_at < ?`,
		householdID, before,
	)
	if err != nil {
		return nil, fmt.Errorf("select old backups: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan s3 key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	_, err = s.db.Exec(
		`DELETE FROM backups WHERE household_id = ? AND created_at < ?`,
		householdID, before,
	)
	if err != nil {
		return nil, fmt.Errorf("delete old backups: %w", err)
	}
	return keys, nil
}

func (s *BackupStore) LatestCompleted(householdID int64) (*model.Backup, error) {
	b := &model.Backup{}
	var errMsg sql.NullString
	var startedAt, completedAt sql.NullTime
	err := s.db.QueryRow(
		`SELECT id, household_id, filename, s3_key, size_bytes, status, error_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE household_id = ? AND status = ? ORDER BY completed_at DESC LIMIT 1`,
		householdID, model.BackupStatusCompleted,
	).Scan(&b.ID, &b.HouseholdID, &b.Filename, &b.S3Key, &b.SizeBytes, &b.Status, &errMsg, &startedAt, &completedAt, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest completed backup: %w", err)
	}
	b.ErrorMessage = errMsg.String
	if startedAt.Valid {
		b.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		b.CompletedAt = &completedAt.Time
	}
	return b, nil
}

func (s *BackupStore) CountByHousehold(householdID int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM backups WHERE household_id = ?`, householdID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count backups: %w", err)
	}
	return count, nil
}

func (s *BackupStore) TotalSizeByHousehold(householdID int64) (int64, error) {
	var total sql.NullInt64
	err := s.db.QueryRow(
		`SELECT SUM(size_bytes) FROM backups WHERE household_id = ? AND status = ?`,
		householdID, model.BackupStatusCompleted,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total backup size: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}
