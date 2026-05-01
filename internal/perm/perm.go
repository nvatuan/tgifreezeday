package perm

import "strings"

// Role represents a user's permission level.
type Role string

const (
	RolePower    Role = "power"
	RoleWrite    Role = "write"
	RoleReadOnly Role = "readonly"
)

// Resolver determines a user's Role from their email address.
// Lists are loaded from POWER_USER_EMAIL_LIST and WRITE_USER_EMAIL_LIST env vars.
type Resolver struct {
	powerUsers map[string]bool
	writeUsers map[string]bool
}

// New creates a Resolver from comma-separated email lists.
func New(powerList, writeList string) *Resolver {
	return &Resolver{
		powerUsers: parseList(powerList),
		writeUsers: parseList(writeList),
	}
}

func parseList(s string) map[string]bool {
	m := make(map[string]bool)
	for _, e := range strings.Split(s, ",") {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" {
			m[e] = true
		}
	}
	return m
}

// RoleFor returns the Role for the given email address.
func (r *Resolver) RoleFor(email string) Role {
	e := strings.ToLower(email)
	if r.powerUsers[e] {
		return RolePower
	}
	if r.writeUsers[e] {
		return RoleWrite
	}
	return RoleReadOnly
}

// CanCreate returns true if this role can create new configs.
func (role Role) CanCreate() bool {
	return role == RolePower || role == RoleWrite
}

// CanEditConfig returns true if this role can edit or delete the given config.
// Power users can edit any config; write users only their own.
func (role Role) CanEditConfig(configOwnerID, currentUserID int64) bool {
	switch role {
	case RolePower:
		return true
	case RoleWrite:
		return configOwnerID == currentUserID
	default:
		return false
	}
}

// CanSyncConfig returns true if this role can sync/wipe/validate the given config.
func (role Role) CanSyncConfig(configOwnerID, currentUserID int64) bool {
	return role.CanEditConfig(configOwnerID, currentUserID)
}

// DisplayName returns a human-readable label for the role.
func (role Role) DisplayName() string {
	switch role {
	case RolePower:
		return "Power User"
	case RoleWrite:
		return "Write User"
	default:
		return "Read Only"
	}
}

// WelcomeMessage returns the post-login notification text for this role.
func (role Role) WelcomeMessage() string {
	switch role {
	case RolePower:
		return "You have full access — you can create, edit, and manage all configs."
	case RoleWrite:
		return "You can create configs and manage your own. Others' configs are read-only."
	default:
		return "You have read-only access. Contact an admin to request write permissions."
	}
}
