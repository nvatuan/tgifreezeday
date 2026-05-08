package db

import (
	"database/sql"
	"errors"
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

const (
	SyncScheduleNone    = "none"
	SyncScheduleWeekly  = "weekly"
	SyncScheduleMonthly = "monthly"
)

type Config struct {
	ID                 int64
	UserID             int64
	Name               string
	SchemaVersion      string
	ConfigYAML         string
	Status             ConfigStatus
	StatusMessage      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	SyncSchedule       string
	NextSyncAt         *time.Time
	LastAutoSyncedAt   *time.Time
	LastAutoSyncResult *string
}

// ConfigWithAuthor enriches Config with the owning user's display info.
type ConfigWithAuthor struct {
	Config
	AuthorEmail       string
	AuthorDisplayName string
}

type ConfigStore struct{ db *sql.DB }

func NewConfigStore(db *sql.DB) *ConfigStore { return &ConfigStore{db: db} }

const configSelectCols = `id, user_id, name, schema_version, config_yaml,
	status, status_message, created_at, updated_at,
	sync_schedule, next_sync_at, last_auto_synced_at, last_auto_sync_result`

func scanConfig(row interface{ Scan(dest ...any) error }) (*Config, error) {
	c := &Config{}
	var nextSyncAt sql.NullTime
	var lastAutoSyncedAt sql.NullTime
	var lastAutoSyncResult sql.NullString
	err := row.Scan(
		&c.ID, &c.UserID, &c.Name, &c.SchemaVersion, &c.ConfigYAML,
		&c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt,
		&c.SyncSchedule, &nextSyncAt, &lastAutoSyncedAt, &lastAutoSyncResult,
	)
	if err != nil {
		return nil, err
	}
	if nextSyncAt.Valid {
		c.NextSyncAt = &nextSyncAt.Time
	}
	if lastAutoSyncedAt.Valid {
		c.LastAutoSyncedAt = &lastAutoSyncedAt.Time
	}
	if lastAutoSyncResult.Valid {
		c.LastAutoSyncResult = &lastAutoSyncResult.String
	}
	return c, nil
}

// ListAllWithAuthor returns all configs joined with their authors.
// Pass a non-nil filterUserID to restrict to a single user.
func (s *ConfigStore) ListAllWithAuthor(filterUserID *int64) ([]*ConfigWithAuthor, error) {
	query := `
		SELECT c.id, c.user_id, c.name, c.schema_version, c.config_yaml,
		       c.status, c.status_message, c.created_at, c.updated_at,
		       c.sync_schedule, c.next_sync_at, c.last_auto_synced_at, c.last_auto_sync_result,
		       u.email, u.display_name
		FROM configs c
		JOIN users u ON c.user_id = u.id`
	var args []interface{}
	if filterUserID != nil {
		query += ` WHERE c.user_id = ?`
		args = append(args, *filterUserID)
	}
	query += ` ORDER BY c.updated_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list all configs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []*ConfigWithAuthor
	for rows.Next() {
		r := &ConfigWithAuthor{}
		var nextSyncAt sql.NullTime
		var lastAutoSyncedAt sql.NullTime
		var lastAutoSyncResult sql.NullString
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Name, &r.SchemaVersion, &r.ConfigYAML,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt,
			&r.SyncSchedule, &nextSyncAt, &lastAutoSyncedAt, &lastAutoSyncResult,
			&r.AuthorEmail, &r.AuthorDisplayName,
		); err != nil {
			return nil, err
		}
		if nextSyncAt.Valid {
			r.NextSyncAt = &nextSyncAt.Time
		}
		if lastAutoSyncedAt.Valid {
			r.LastAutoSyncedAt = &lastAutoSyncedAt.Time
		}
		if lastAutoSyncResult.Valid {
			r.LastAutoSyncResult = &lastAutoSyncResult.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *ConfigStore) Create(userID int64, name, schemaVersion, configYAML, syncSchedule string, nextSyncAt *time.Time) (*Config, error) {
	res, err := s.db.Exec(`
		INSERT INTO configs (user_id, name, schema_version, config_yaml, status, sync_schedule, next_sync_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)
	`, userID, name, schemaVersion, configYAML, syncSchedule, nextSyncAt)
	if err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id, userID)
}

func (s *ConfigStore) Get(id, userID int64) (*Config, error) {
	row := s.db.QueryRow(`
		SELECT `+configSelectCols+`
		FROM configs WHERE id = ? AND user_id = ?
	`, id, userID)
	c, err := scanConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	return c, nil
}

// GetByID fetches a config by ID without scoping by user. Use only for power-user access.
func (s *ConfigStore) GetByID(id int64) (*Config, error) {
	row := s.db.QueryRow(`
		SELECT `+configSelectCols+`
		FROM configs WHERE id = ?
	`, id)
	c, err := scanConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get config by id: %w", err)
	}
	return c, nil
}

func (s *ConfigStore) ListByUser(userID int64) ([]*Config, error) {
	rows, err := s.db.Query(`
		SELECT `+configSelectCols+`
		FROM configs WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var configs []*Config
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (s *ConfigStore) Update(id, userID int64, name, configYAML, syncSchedule string, nextSyncAt *time.Time) error {
	_, err := s.db.Exec(`
		UPDATE configs
		SET name = ?, config_yaml = ?, status = 'pending', status_message = '',
		    sync_schedule = ?, next_sync_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, name, configYAML, syncSchedule, nextSyncAt, id, userID)
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

// ListDueForAutoSync returns configs whose auto-sync schedule is due (next_sync_at <= now).
func (s *ConfigStore) ListDueForAutoSync(now time.Time) ([]*Config, error) {
	rows, err := s.db.Query(`
		SELECT `+configSelectCols+`
		FROM configs
		WHERE sync_schedule != 'none' AND next_sync_at IS NOT NULL AND next_sync_at <= ?
		ORDER BY next_sync_at ASC
	`, now)
	if err != nil {
		return nil, fmt.Errorf("list due auto-sync configs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var configs []*Config
	for rows.Next() {
		c, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// RecordAutoSync stores the result of an auto-sync run and advances next_sync_at.
func (s *ConfigStore) RecordAutoSync(id int64, syncedAt time.Time, result string, nextSyncAt time.Time) error {
	_, err := s.db.Exec(`
		UPDATE configs
		SET last_auto_synced_at = ?, last_auto_sync_result = ?, next_sync_at = ?
		WHERE id = ?
	`, syncedAt, result, nextSyncAt, id)
	return err
}
