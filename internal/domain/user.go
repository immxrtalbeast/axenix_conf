package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents a participant profile that can join rooms.
type User struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	IsGuest   bool      `json:"is_guest"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewGuestUser(name string) *User {
	return &User{
		ID:        uuid.New(),
		Name:      name,
		IsGuest:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func NewUser(name string, email string) *User {
	now := time.Now().UTC()
	return &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
