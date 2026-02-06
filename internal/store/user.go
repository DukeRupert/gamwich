package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/model"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func scanUser(scanner interface{ Scan(...any) error }) (*model.User, error) {
	var u model.User
	err := scanner.Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

const userCols = `id, email, name, created_at, updated_at`

func (s *UserStore) Create(email, name string) (*model.User, error) {
	result, err := s.db.Exec(
		`INSERT INTO users (email, name) VALUES (?, ?)`,
		email, name,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *UserStore) GetByID(id int64) (*model.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *UserStore) GetByEmail(email string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE email = ?`, email)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (s *UserStore) Update(id int64, email, name string) (*model.User, error) {
	_, err := s.db.Exec(
		`UPDATE users SET email = ?, name = ? WHERE id = ?`,
		email, name, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return s.GetByID(id)
}

func (s *UserStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
