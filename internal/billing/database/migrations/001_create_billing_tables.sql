-- +goose Up

CREATE TABLE accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    stripe_customer_id TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose StatementBegin
CREATE TRIGGER update_accounts_updated_at
AFTER UPDATE ON accounts
FOR EACH ROW
BEGIN
    UPDATE accounts SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    stripe_subscription_id TEXT,
    plan TEXT NOT NULL DEFAULT 'cloud',
    status TEXT NOT NULL DEFAULT 'active',
    current_period_end DATETIME,
    cancel_at_period_end INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_subscriptions_account_id ON subscriptions(account_id);
CREATE INDEX idx_subscriptions_stripe_subscription_id ON subscriptions(stripe_subscription_id);

-- +goose StatementBegin
CREATE TRIGGER update_subscriptions_updated_at
AFTER UPDATE ON subscriptions
FOR EACH ROW
BEGIN
    UPDATE subscriptions SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE license_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    key TEXT NOT NULL UNIQUE,
    plan TEXT NOT NULL DEFAULT 'cloud',
    features TEXT NOT NULL DEFAULT '',
    activated_at DATETIME,
    expires_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_license_keys_account_id ON license_keys(account_id);
CREATE INDEX idx_license_keys_subscription_id ON license_keys(subscription_id);
CREATE INDEX idx_license_keys_key ON license_keys(key);

-- +goose StatementBegin
CREATE TRIGGER update_license_keys_updated_at
AFTER UPDATE ON license_keys
FOR EACH ROW
BEGIN
    UPDATE license_keys SET updated_at = datetime('now') WHERE id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_account_id ON sessions(account_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- +goose Down
DROP TABLE IF EXISTS sessions;
DROP TRIGGER IF EXISTS update_license_keys_updated_at;
DROP TABLE IF EXISTS license_keys;
DROP TRIGGER IF EXISTS update_subscriptions_updated_at;
DROP TABLE IF EXISTS subscriptions;
DROP TRIGGER IF EXISTS update_accounts_updated_at;
DROP TABLE IF EXISTS accounts;
