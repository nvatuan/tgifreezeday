package db

import (
	"database/sql"
	"fmt"
	"time"
)

type User struct {
	ID          int64
	GoogleID    string
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

// Upsert inserts or updates the user by google_id. Returns the user with its DB id.
func (s *UserStore) Upsert(googleID, email, displayName string) (*User, error) {
	_, err := s.db.Exec(`
		INSERT INTO users (google_id, email, display_name)
		VALUES (?, ?, ?)
		ON CONFLICT(google_id) DO UPDATE SET
			email        = excluded.email,
			display_name = excluded.display_name
	`, googleID, email, displayName)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	return s.GetByGoogleID(googleID)
}

func (s *UserStore) GetByID(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, google_id, email, display_name, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.GoogleID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// ListAll returns every user ordered by display_name.
func (s *UserStore) ListAll() ([]*User, error) {
	rows, err := s.db.Query(
		`SELECT id, google_id, email, display_name, created_at FROM users ORDER BY display_name, email`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.GoogleID, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) GetByGoogleID(googleID string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		`SELECT id, google_id, email, display_name, created_at FROM users WHERE google_id = ?`, googleID,
	).Scan(&u.ID, &u.GoogleID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by google_id: %w", err)
	}
	return u, nil
}
