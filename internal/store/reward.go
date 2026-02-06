package store

import (
	"database/sql"
	"fmt"

	"github.com/dukerupert/gamwich/internal/model"
)

type RewardStore struct {
	db *sql.DB
}

func NewRewardStore(db *sql.DB) *RewardStore {
	return &RewardStore{db: db}
}

// --- Reward methods ---

func scanReward(scanner interface{ Scan(...any) error }) (*model.Reward, error) {
	var r model.Reward
	var active int

	err := scanner.Scan(&r.ID, &r.Title, &r.Description, &r.PointCost, &active, &r.CreatedAt)
	if err != nil {
		return nil, err
	}

	r.Active = active != 0
	return &r, nil
}

const rewardCols = `id, title, description, point_cost, active, created_at`

func (s *RewardStore) Create(title, description string, pointCost int, active bool) (*model.Reward, error) {
	var a int
	if active {
		a = 1
	}

	result, err := s.db.Exec(
		`INSERT INTO rewards (title, description, point_cost, active) VALUES (?, ?, ?, ?)`,
		title, description, pointCost, a,
	)
	if err != nil {
		return nil, fmt.Errorf("insert reward: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(id)
}

func (s *RewardStore) GetByID(id int64) (*model.Reward, error) {
	row := s.db.QueryRow(`SELECT `+rewardCols+` FROM rewards WHERE id = ?`, id)
	r, err := scanReward(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reward: %w", err)
	}
	return r, nil
}

// List returns all rewards, active first, then by title.
func (s *RewardStore) List() ([]model.Reward, error) {
	rows, err := s.db.Query(`SELECT ` + rewardCols + ` FROM rewards ORDER BY active DESC, title ASC`)
	if err != nil {
		return nil, fmt.Errorf("list rewards: %w", err)
	}
	defer rows.Close()

	var rewards []model.Reward
	for rows.Next() {
		r, err := scanReward(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reward: %w", err)
		}
		rewards = append(rewards, *r)
	}
	return rewards, rows.Err()
}

// ListActive returns only active rewards, ordered by title.
func (s *RewardStore) ListActive() ([]model.Reward, error) {
	rows, err := s.db.Query(`SELECT ` + rewardCols + ` FROM rewards WHERE active = 1 ORDER BY title ASC`)
	if err != nil {
		return nil, fmt.Errorf("list active rewards: %w", err)
	}
	defer rows.Close()

	var rewards []model.Reward
	for rows.Next() {
		r, err := scanReward(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reward: %w", err)
		}
		rewards = append(rewards, *r)
	}
	return rewards, rows.Err()
}

func (s *RewardStore) Update(id int64, title, description string, pointCost int, active bool) (*model.Reward, error) {
	var a int
	if active {
		a = 1
	}

	_, err := s.db.Exec(
		`UPDATE rewards SET title = ?, description = ?, point_cost = ?, active = ? WHERE id = ?`,
		title, description, pointCost, a, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update reward: %w", err)
	}
	return s.GetByID(id)
}

func (s *RewardStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM rewards WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete reward: %w", err)
	}
	return nil
}

// --- Redemption methods ---

func scanRedemption(scanner interface{ Scan(...any) error }) (*model.RewardRedemption, error) {
	var r model.RewardRedemption
	var redeemedBy sql.NullInt64

	err := scanner.Scan(&r.ID, &r.RewardID, &redeemedBy, &r.PointsSpent, &r.RedeemedAt)
	if err != nil {
		return nil, err
	}

	if redeemedBy.Valid {
		r.RedeemedBy = &redeemedBy.Int64
	}
	return &r, nil
}

const redemptionCols = `id, reward_id, redeemed_by, points_spent, redeemed_at`

func (s *RewardStore) Redeem(rewardID int64, redeemedBy *int64, pointsSpent int) (*model.RewardRedemption, error) {
	var rBy sql.NullInt64
	if redeemedBy != nil {
		rBy = sql.NullInt64{Int64: *redeemedBy, Valid: true}
	}

	result, err := s.db.Exec(
		`INSERT INTO reward_redemptions (reward_id, redeemed_by, points_spent) VALUES (?, ?, ?)`,
		rewardID, rBy, pointsSpent,
	)
	if err != nil {
		return nil, fmt.Errorf("insert redemption: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	row := s.db.QueryRow(`SELECT `+redemptionCols+` FROM reward_redemptions WHERE id = ?`, id)
	return scanRedemption(row)
}

func (s *RewardStore) ListRedemptionsByMember(memberID int64) ([]model.RewardRedemption, error) {
	rows, err := s.db.Query(
		`SELECT `+redemptionCols+` FROM reward_redemptions WHERE redeemed_by = ? ORDER BY redeemed_at DESC`,
		memberID,
	)
	if err != nil {
		return nil, fmt.Errorf("list redemptions by member: %w", err)
	}
	defer rows.Close()

	var redemptions []model.RewardRedemption
	for rows.Next() {
		r, err := scanRedemption(rows)
		if err != nil {
			return nil, fmt.Errorf("scan redemption: %w", err)
		}
		redemptions = append(redemptions, *r)
	}
	return redemptions, rows.Err()
}

// --- Point balance methods ---

// GetPointBalance computes the balance for a single member: earned - spent.
func (s *RewardStore) GetPointBalance(memberID int64) (*model.PointBalance, error) {
	var earned sql.NullInt64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(points_earned), 0) FROM chore_completions WHERE completed_by = ?`,
		memberID,
	).Scan(&earned)
	if err != nil {
		return nil, fmt.Errorf("sum points earned: %w", err)
	}

	var spent sql.NullInt64
	err = s.db.QueryRow(
		`SELECT COALESCE(SUM(points_spent), 0) FROM reward_redemptions WHERE redeemed_by = ?`,
		memberID,
	).Scan(&spent)
	if err != nil {
		return nil, fmt.Errorf("sum points spent: %w", err)
	}

	// Get member name
	var name string
	err = s.db.QueryRow(`SELECT name FROM family_members WHERE id = ?`, memberID).Scan(&name)
	if err == sql.ErrNoRows {
		name = "Unknown"
	} else if err != nil {
		return nil, fmt.Errorf("get member name: %w", err)
	}

	totalEarned := int(earned.Int64)
	totalSpent := int(spent.Int64)

	return &model.PointBalance{
		MemberID:    memberID,
		MemberName:  name,
		TotalEarned: totalEarned,
		TotalSpent:  totalSpent,
		Balance:     totalEarned - totalSpent,
	}, nil
}

// GetAllPointBalances returns point balances for all family members, ordered by balance DESC.
func (s *RewardStore) GetAllPointBalances() ([]model.PointBalance, error) {
	rows, err := s.db.Query(`SELECT id, name FROM family_members ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	type memberInfo struct {
		ID   int64
		Name string
	}
	var members []memberInfo
	for rows.Next() {
		var m memberInfo
		if err := rows.Scan(&m.ID, &m.Name); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	rows.Close()

	var balances []model.PointBalance
	for _, m := range members {
		var earned int
		if err := s.db.QueryRow(
			`SELECT COALESCE(SUM(points_earned), 0) FROM chore_completions WHERE completed_by = ?`,
			m.ID,
		).Scan(&earned); err != nil {
			return nil, fmt.Errorf("sum points earned for member %d: %w", m.ID, err)
		}

		var spent int
		if err := s.db.QueryRow(
			`SELECT COALESCE(SUM(points_spent), 0) FROM reward_redemptions WHERE redeemed_by = ?`,
			m.ID,
		).Scan(&spent); err != nil {
			return nil, fmt.Errorf("sum points spent for member %d: %w", m.ID, err)
		}

		balances = append(balances, model.PointBalance{
			MemberID:    m.ID,
			MemberName:  m.Name,
			TotalEarned: earned,
			TotalSpent:  spent,
			Balance:     earned - spent,
		})
	}

	// Sort by balance DESC
	for i := 0; i < len(balances); i++ {
		for j := i + 1; j < len(balances); j++ {
			if balances[j].Balance > balances[i].Balance {
				balances[i], balances[j] = balances[j], balances[i]
			}
		}
	}

	return balances, rows.Err()
}
