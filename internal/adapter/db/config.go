package db

import (
	"database/sql"
	"fmt"
	"time"
)

type ConfigStatus string

const (
	ConfigStatusPending      ConfigStatus = "pending"
	ConfigStatusValid        ConfigStatus = "valid"
	ConfigStatusInvalid      ConfigStatus = "invalid"
	ConfigStatusUnauthorized ConfigStatus = "unauthorized"
)

type Config struct {
	ID            int64
	UserID        int64
	Name          string
	SchemaVersion string
	ConfigYAML    string
	Status        ConfigStatus
	StatusMessage string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ConfigStore struct{ db *sql.DB }

func NewConfigStore(db *sql.DB) *ConfigStore { return &ConfigStore{db: db} }

func (s *ConfigStore) Create(userID int64, name, schemaVersion, configYAML string) (*Config, error) {
	res, err := s.db.Exec(`
		INSERT INTO configs (user_id, name, schema_version, config_yaml, status)
		VALUES (?, ?, ?, ?, 'pending')
	`, userID, name, schemaVersion, configYAML)
	if err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id, userID)
}

func (s *ConfigStore) Get(id, userID int64) (*Config, error) {
	c := &Config{}
	err := s.db.QueryRow(`
		SELECT id, user_id, name, schema_version, config_yaml, status, status_message, created_at, updated_at
		FROM configs WHERE id = ? AND user_id = ?
	`, id, userID).Scan(
		&c.ID, &c.UserID, &c.Name, &c.SchemaVersion, &c.ConfigYAML,
		&c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	return c, nil
}

func (s *ConfigStore) ListByUser(userID int64) ([]*Config, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, schema_version, config_yaml, status, status_message, created_at, updated_at
		FROM configs WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	var configs []*Config
	for rows.Next() {
		c := &Config{}
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Name, &c.SchemaVersion, &c.ConfigYAML,
			&c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (s *ConfigStore) Update(id, userID int64, name, configYAML string) error {
	_, err := s.db.Exec(`
		UPDATE configs SET name = ?, config_yaml = ?, status = 'pending', status_message = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, name, configYAML, id, userID)
	return err
}

func (s *ConfigStore) UpdateStatus(id int64, status ConfigStatus, message string) error {
	_, err := s.db.Exec(`
		UPDATE configs SET status = ?, status_message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?
	`, status, message, id)
	return err
}

func (s *ConfigStore) Delete(id, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM configs WHERE id = ? AND user_id = ?`, id, userID)
	return err
}
