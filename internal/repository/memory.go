package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
)

var (
	ErrRoomNotFound    = errors.New("room not found")
	ErrRoomLinkExists  = errors.New("room link already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserEmailExists = errors.New("user with email already exists")
)

type InMemoryRoomRepository struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]*domain.Room
	links map[string]uuid.UUID
}

func NewInMemoryRoomRepository() *InMemoryRoomRepository {
	return &InMemoryRoomRepository{
		rooms: make(map[uuid.UUID]*domain.Room),
		links: make(map[string]uuid.UUID),
	}
}

func (r *InMemoryRoomRepository) Create(ctx context.Context, room *domain.Room) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.links[room.Link]; ok {
		return ErrRoomLinkExists
	}

	r.rooms[room.ID] = room
	r.links[room.Link] = room.ID
	return nil
}

func (r *InMemoryRoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	room, ok := r.rooms[id]
	if !ok {
		return nil, ErrRoomNotFound
	}

	return room, nil
}

func (r *InMemoryRoomRepository) GetByLink(ctx context.Context, link string) (*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	roomID, ok := r.links[link]
	if !ok {
		return nil, ErrRoomNotFound
	}

	room, ok := r.rooms[roomID]
	if !ok {
		return nil, ErrRoomNotFound
	}

	return room, nil
}

func (r *InMemoryRoomRepository) Update(ctx context.Context, room *domain.Room) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.rooms[room.ID]; !ok {
		return ErrRoomNotFound
	}

	r.rooms[room.ID] = room
	r.links[room.Link] = room.ID
	return nil
}

func (r *InMemoryRoomRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	room, ok := r.rooms[id]
	if !ok {
		return ErrRoomNotFound
	}

	delete(r.links, room.Link)
	delete(r.rooms, id)
	return nil
}

func (r *InMemoryRoomRepository) List(ctx context.Context) ([]*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		result = append(result, room)
	}
	return result, nil
}

type InMemoryUserRepository struct {
	mu     sync.RWMutex
	users  map[uuid.UUID]*domain.User
	emails map[string]uuid.UUID
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users:  make(map[uuid.UUID]*domain.User),
		emails: make(map[string]uuid.UUID),
	}
}

func (r *InMemoryUserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if user.Email != "" {
		if _, ok := r.emails[user.Email]; ok {
			return ErrUserEmailExists
		}
		r.emails[user.Email] = user.ID
	}

	r.users[user.ID] = user
	return nil
}

func (r *InMemoryUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	return user, nil
}

func (r *InMemoryUserRepository) Update(ctx context.Context, user *domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[user.ID]; !ok {
		return ErrUserNotFound
	}

	r.users[user.ID] = user
	if user.Email != "" {
		r.emails[user.Email] = user.ID
	}
	return nil
}
