package googlecalendar

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// stubTokenSource returns a fixed token on each call.
type stubTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s *stubTokenSource) Token() (*oauth2.Token, error) {
	return s.token, s.err
}

// stubTokenStore records Upsert calls.
type stubTokenStore struct {
	calls []*oauth2.Token
	err   error
}

func (s *stubTokenStore) Upsert(_ int64, t *oauth2.Token) error {
	s.calls = append(s.calls, t)
	return s.err
}

func tokenWithAccess(access string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  access,
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(time.Hour),
	}
}

func TestPersistingTokenSource_PersistsOnRefresh(t *testing.T) {
	initial := tokenWithAccess("old-access")
	refreshed := tokenWithAccess("new-access")

	store := &stubTokenStore{}
	src := &persistingTokenSource{
		base:    &stubTokenSource{token: refreshed},
		userID:  42,
		store:   store,
		current: initial,
	}

	got, err := src.Token()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AccessToken != "new-access" {
		t.Errorf("got access token %q, want %q", got.AccessToken, "new-access")
	}
	if len(store.calls) != 1 {
		t.Errorf("Upsert called %d times, want 1", len(store.calls))
	}
}

func TestPersistingTokenSource_NoPersistWhenTokenUnchanged(t *testing.T) {
	tok := tokenWithAccess("same-access")

	store := &stubTokenStore{}
	src := &persistingTokenSource{
		base:    &stubTokenSource{token: tok},
		userID:  42,
		store:   store,
		current: tok,
	}

	if _, err := src.Token(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.calls) != 0 {
		t.Errorf("Upsert called %d times, want 0", len(store.calls))
	}
}

func TestPersistingTokenSource_PropagatesBaseError(t *testing.T) {
	baseErr := errors.New("refresh failed")
	store := &stubTokenStore{}
	src := &persistingTokenSource{
		base:    &stubTokenSource{err: baseErr},
		userID:  42,
		store:   store,
		current: tokenWithAccess("old"),
	}

	_, err := src.Token()
	if !errors.Is(err, baseErr) {
		t.Errorf("got error %v, want %v", err, baseErr)
	}
	if len(store.calls) != 0 {
		t.Errorf("Upsert should not be called on base error, got %d calls", len(store.calls))
	}
}
