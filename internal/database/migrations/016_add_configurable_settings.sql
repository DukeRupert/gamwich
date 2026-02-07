-- +goose Up

-- Seed email settings for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_postmark_token', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_from_address', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'email_base_url', '' FROM households;

-- Seed S3 settings for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_s3_endpoint', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_s3_bucket', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_s3_region', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_s3_access_key', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'backup_s3_secret_key', '' FROM households;

-- Seed VAPID settings for existing households
INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'vapid_public_key', '' FROM households;

INSERT OR IGNORE INTO settings (household_id, key, value)
SELECT id, 'vapid_private_key', '' FROM households;

-- +goose Down
DELETE FROM settings WHERE key IN (
    'email_postmark_token', 'email_from_address', 'email_base_url',
    'backup_s3_endpoint', 'backup_s3_bucket', 'backup_s3_region', 'backup_s3_access_key', 'backup_s3_secret_key',
    'vapid_public_key', 'vapid_private_key'
);
