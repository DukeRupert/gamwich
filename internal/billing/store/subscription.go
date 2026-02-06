package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/billing/model"
)

type SubscriptionStore struct {
	db *sql.DB
}

func NewSubscriptionStore(db *sql.DB) *SubscriptionStore {
	return &SubscriptionStore{db: db}
}

func scanSubscription(scanner interface{ Scan(...any) error }) (*model.Subscription, error) {
	var sub model.Subscription
	var stripeSubID sql.NullString
	var periodEnd sql.NullTime
	var cancelAtPeriodEnd int
	err := scanner.Scan(
		&sub.ID, &sub.AccountID, &stripeSubID, &sub.Plan, &sub.Status,
		&periodEnd, &cancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if stripeSubID.Valid {
		sub.StripeSubscriptionID = &stripeSubID.String
	}
	if periodEnd.Valid {
		sub.CurrentPeriodEnd = &periodEnd.Time
	}
	sub.CancelAtPeriodEnd = cancelAtPeriodEnd != 0
	return &sub, nil
}

const subscriptionCols = `id, account_id, stripe_subscription_id, plan, status, current_period_end, cancel_at_period_end, created_at, updated_at`

func (s *SubscriptionStore) Create(accountID int64, plan string) (*model.Subscription, error) {
	result, err := s.db.Exec(
		`INSERT INTO subscriptions (account_id, plan) VALUES (?, ?)`,
		accountID, plan,
	)
	if err != nil {
		return nil, fmt.Errorf("insert subscription: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *SubscriptionStore) GetByID(id int64) (*model.Subscription, error) {
	row := s.db.QueryRow(`SELECT `+subscriptionCols+` FROM subscriptions WHERE id = ?`, id)
	sub, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return sub, nil
}

func (s *SubscriptionStore) GetByAccountID(accountID int64) (*model.Subscription, error) {
	row := s.db.QueryRow(
		`SELECT `+subscriptionCols+` FROM subscriptions WHERE account_id = ? ORDER BY created_at DESC LIMIT 1`,
		accountID,
	)
	sub, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get subscription by account: %w", err)
	}
	return sub, nil
}

func (s *SubscriptionStore) GetByStripeID(stripeSubID string) (*model.Subscription, error) {
	row := s.db.QueryRow(
		`SELECT `+subscriptionCols+` FROM subscriptions WHERE stripe_subscription_id = ?`,
		stripeSubID,
	)
	sub, err := scanSubscription(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get subscription by stripe id: %w", err)
	}
	return sub, nil
}

func (s *SubscriptionStore) UpdateStatus(id int64, status string) error {
	_, err := s.db.Exec(
		`UPDATE subscriptions SET status = ? WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}
	return nil
}

func (s *SubscriptionStore) UpdateStripeID(id int64, stripeSubID string) error {
	_, err := s.db.Exec(
		`UPDATE subscriptions SET stripe_subscription_id = ? WHERE id = ?`,
		stripeSubID, id,
	)
	if err != nil {
		return fmt.Errorf("update stripe subscription id: %w", err)
	}
	return nil
}

func (s *SubscriptionStore) SetCancelAtPeriodEnd(id int64, cancel bool) error {
	var v int
	if cancel {
		v = 1
	}
	_, err := s.db.Exec(
		`UPDATE subscriptions SET cancel_at_period_end = ? WHERE id = ?`,
		v, id,
	)
	if err != nil {
		return fmt.Errorf("set cancel at period end: %w", err)
	}
	return nil
}

func (s *SubscriptionStore) UpdatePeriodEnd(id int64, periodEnd *sql.NullTime) error {
	_, err := s.db.Exec(
		`UPDATE subscriptions SET current_period_end = ? WHERE id = ?`,
		periodEnd, id,
	)
	if err != nil {
		return fmt.Errorf("update period end: %w", err)
	}
	return nil
}

func (s *SubscriptionStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM subscriptions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}
