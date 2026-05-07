package db_test

import (
	"testing"

	"github.com/nvat/tgifreezeday/internal/adapter/db"
)

func TestConfigStore_GetByID_CrossUser(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close() //nolint:errcheck

	users := db.NewUserStore(database)
	configs := db.NewConfigStore(database)

	user1, err := users.Upsert("google-1", "user1@example.com", "User One")
	if err != nil {
		t.Fatalf("upsert user1: %v", err)
	}
	user2, err := users.Upsert("google-2", "user2@example.com", "User Two")
	if err != nil {
		t.Fatalf("upsert user2: %v", err)
	}

	cfg, err := configs.Create(user1.ID, "Test Config", "v1", "shared:\n  lookbackDays: 7\n")
	if err != nil {
		t.Fatalf("create config: %v", err)
	}

	// Scoped Get by owner works.
	got, err := configs.Get(cfg.ID, user1.ID)
	if err != nil || got == nil {
		t.Fatal("expected config for owner, got nil")
	}

	// Scoped Get by non-owner returns nil — power users are blocked by this.
	got, err = configs.Get(cfg.ID, user2.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for non-owner scoped Get")
	}

	// GetByID returns config regardless of owner.
	got, err = configs.GetByID(cfg.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got == nil {
		t.Fatal("expected config from GetByID, got nil")
	}
	if got.ID != cfg.ID {
		t.Fatalf("expected id %d, got %d", cfg.ID, got.ID)
	}
}

func TestConfigStore_GetByID_NotFound(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close() //nolint:errcheck

	configs := db.NewConfigStore(database)

	got, err := configs.GetByID(99999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for missing config")
	}
}
