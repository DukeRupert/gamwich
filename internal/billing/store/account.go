package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/billing/model"
)

type AccountStore struct {
	db *sql.DB
}

func NewAccountStore(db *sql.DB) *AccountStore {
	return &AccountStore{db: db}
}

func scanAccount(scanner interface{ Scan(...any) error }) (*model.Account, error) {
	var a model.Account
	var stripeID sql.NullString
	err := scanner.Scan(&a.ID, &a.Email, &stripeID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if stripeID.Valid {
		a.StripeCustomerID = &stripeID.String
	}
	return &a, nil
}

const accountCols = `id, email, stripe_customer_id, created_at, updated_at`

func (s *AccountStore) Create(email string) (*model.Account, error) {
	result, err := s.db.Exec(
		`INSERT INTO accounts (email) VALUES (?)`,
		email,
	)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *AccountStore) GetByID(id int64) (*model.Account, error) {
	row := s.db.QueryRow(`SELECT `+accountCols+` FROM accounts WHERE id = ?`, id)
	a, err := scanAccount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

func (s *AccountStore) GetByEmail(email string) (*model.Account, error) {
	row := s.db.QueryRow(`SELECT `+accountCols+` FROM accounts WHERE email = ?`, email)
	a, err := scanAccount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get account by email: %w", err)
	}
	return a, nil
}

func (s *AccountStore) UpdateStripeCustomerID(id int64, customerID string) error {
	_, err := s.db.Exec(
		`UPDATE accounts SET stripe_customer_id = ? WHERE id = ?`,
		customerID, id,
	)
	if err != nil {
		return fmt.Errorf("update stripe customer id: %w", err)
	}
	return nil
}

func (s *AccountStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	return nil
}
