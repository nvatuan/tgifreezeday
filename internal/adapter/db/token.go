package db

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/oauth2"
)

type TokenStore struct{ db *sql.DB }

func NewTokenStore(db *sql.DB) *TokenStore { return &TokenStore{db: db} }

func (s *TokenStore) Upsert(userID int64, token *oauth2.Token) error {
	_, err := s.db.Exec(`
		INSERT INTO oauth_tokens (user_id, access_token, token_type, refresh_token, expiry, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			access_token  = excluded.access_token,
			token_type    = excluded.token_type,
			refresh_token = excluded.refresh_token,
			expiry        = excluded.expiry,
			updated_at    = CURRENT_TIMESTAMP
	`, userID, token.AccessToken, token.TokenType, token.RefreshToken, token.Expiry)
	if err != nil {
		return fmt.Errorf("upsert token: %w", err)
	}
	return nil
}

func (s *TokenStore) Get(userID int64) (*oauth2.Token, error) {
	var accessToken, tokenType, refreshToken string
	var expiry sql.NullTime
	err := s.db.QueryRow(`
		SELECT access_token, token_type, refresh_token, expiry
		FROM oauth_tokens WHERE user_id = ?
	`, userID).Scan(&accessToken, &tokenType, &refreshToken, &expiry)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	t := &oauth2.Token{
		AccessToken:  accessToken,
		TokenType:    tokenType,
		RefreshToken: refreshToken,
	}
	if expiry.Valid {
		t.Expiry = expiry.Time
	} else {
		t.Expiry = time.Time{}
	}
	return t, nil
}
