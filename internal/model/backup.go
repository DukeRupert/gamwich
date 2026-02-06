package model

import "time"

type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"
	BackupStatusUploading BackupStatus = "uploading"
	BackupStatusCompleted BackupStatus = "completed"
	BackupStatusFailed    BackupStatus = "failed"
)

type Backup struct {
	ID           int64        `json:"id"`
	HouseholdID  int64        `json:"household_id"`
	Filename     string       `json:"filename"`
	S3Key        string       `json:"s3_key"`
	SizeBytes    int64        `json:"size_bytes"`
	Status       BackupStatus `json:"status"`
	ErrorMessage string       `json:"error_message,omitempty"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}
