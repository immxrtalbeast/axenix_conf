package model

import (
	"time"

	"github.com/google/uuid"
)

type Room struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Name      string     `gorm:"size:255;not null"`
	Owner     uuid.UUID  `gorm:"type:uuid;not null"`
	Link      string     `gorm:"size:64;uniqueIndex;not null"`
	CreatedAt time.Time  `gorm:"not null"`
	ExpiresAt *time.Time `gorm:"index"`
	Peers     []Peer     `gorm:"constraint:OnDelete:CASCADE"`
}

type Peer struct {
	ID          string    `gorm:"size:64;primaryKey"`
	RoomID      uuid.UUID `gorm:"type:uuid;index;not null"`
	UserID      uuid.UUID `gorm:"type:uuid;index;not null"`
	DisplayName string    `gorm:"size:255;not null"`
	Status      string    `gorm:"size:32;not null"`
	JoinedAt    time.Time `gorm:"not null"`
	LastSeen    time.Time `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"size:255;not null"`
	Email     *string   `gorm:"size:255;uniqueIndex:idx_users_email,where:email IS NOT NULL"`
	IsGuest   bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}
