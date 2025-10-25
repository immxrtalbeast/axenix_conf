package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
)

type RoomInteractor interface {
	CreateRoom(ctx context.Context, name string, owner uuid.UUID, lifetime time.Duration) (*domain.Room, error)
	GetRoom(ctx context.Context, id uuid.UUID) (*domain.Room, error)
	GetRoomByLink(ctx context.Context, link string) (*domain.Room, error)
	RegisterPeer(ctx context.Context, roomID uuid.UUID, user *domain.User) (*domain.Peer, error)
	UnregisterPeer(ctx context.Context, roomID uuid.UUID, peerID string) error
	HandleSignal(ctx context.Context, roomID uuid.UUID, peerID string, message *domain.SignalMessage) (*domain.SignalMessage, error)
	ListParticipants(ctx context.Context, roomID uuid.UUID) ([]*domain.User, error)
}

type UserInteractor interface {
	CreateUser(ctx context.Context, name string, email string) (*domain.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
}
