package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
	"github.com/immxrtalbeast/axenix_conf/internal/repository/model"
	"gorm.io/gorm"
)

var (
	ErrRoomNotFound    = errors.New("room not found")
	ErrRoomLinkExists  = errors.New("room link already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserEmailExists = errors.New("user with email already exists")
)

type PostgresRoomRepository struct {
	db *gorm.DB
}

func NewPostgresRoomRepository(db *gorm.DB) *PostgresRoomRepository {
	return &PostgresRoomRepository{db: db}
}

func (r *PostgresRoomRepository) Create(ctx context.Context, room *domain.Room) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if room == nil {
		return errors.New("room is nil")
	}

	roomModel := toModelRoom(room)

	if err := r.db.WithContext(ctx).Create(roomModel).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrRoomLinkExists
		}
		return err
	}
	return nil
}

func (r *PostgresRoomRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var room model.Room
	err := r.db.WithContext(ctx).Preload("Peers").First(&room, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	return toDomainRoom(&room), nil
}

func (r *PostgresRoomRepository) GetByLink(ctx context.Context, link string) (*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var room model.Room
	err := r.db.WithContext(ctx).Preload("Peers").First(&room, "link = ?", link).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	return toDomainRoom(&room), nil
}

func (r *PostgresRoomRepository) Update(ctx context.Context, room *domain.Room) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if room == nil {
		return errors.New("room is nil")
	}

	roomModel := toModelRoom(room)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"name":  roomModel.Name,
			"owner": roomModel.Owner,
			"link":  roomModel.Link,
		}

		if roomModel.ExpiresAt == nil {
			updates["expires_at"] = gorm.Expr("NULL")
		} else {
			updates["expires_at"] = roomModel.ExpiresAt
		}

		res := tx.Model(&model.Room{}).Where("id = ?", roomModel.ID).Updates(updates)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
				return ErrRoomLinkExists
			}
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrRoomNotFound
		}

		if err := tx.Where("room_id = ?", roomModel.ID).Delete(&model.Peer{}).Error; err != nil {
			return err
		}

		if len(roomModel.Peers) > 0 {
			if err := tx.Create(&roomModel.Peers).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *PostgresRoomRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	res := r.db.WithContext(ctx).Delete(&model.Room{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRoomNotFound
	}
	return nil
}

func (r *PostgresRoomRepository) List(ctx context.Context) ([]*domain.Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var rooms []model.Room
	if err := r.db.WithContext(ctx).Preload("Peers").Find(&rooms).Error; err != nil {
		return nil, err
	}

	result := make([]*domain.Room, 0, len(rooms))
	for i := range rooms {
		result = append(result, toDomainRoom(&rooms[i]))
	}

	return result, nil
}

type PostgresUserRepository struct {
	db *gorm.DB
}

func NewPostgresUserRepository(db *gorm.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if user == nil {
		return errors.New("user is nil")
	}

	userModel := toModelUser(user)

	if err := r.db.WithContext(ctx).Create(userModel).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrUserEmailExists
		}
		return err
	}
	return nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var user model.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return toDomainUser(&user), nil
}

func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if user == nil {
		return errors.New("user is nil")
	}

	userModel := toModelUser(user)

	updateData := map[string]any{
		"name":       userModel.Name,
		"is_guest":   userModel.IsGuest,
		"updated_at": userModel.UpdatedAt,
	}

	if userModel.Email == nil {
		updateData["email"] = gorm.Expr("NULL")
	} else {
		updateData["email"] = userModel.Email
	}

	res := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userModel.ID).Updates(updateData)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
			return ErrUserEmailExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func toModelRoom(room *domain.Room) *model.Room {
	var expiresAt *time.Time
	if !room.ExpiresAt.IsZero() {
		t := room.ExpiresAt.UTC()
		expiresAt = &t
	}

	peers := make([]model.Peer, 0, len(room.Peers))
	for _, p := range room.Peers {
		if p == nil {
			continue
		}
		status := p.Status
		if status == "" {
			status = domain.PeerStatusConnected
		}
		joinedAt := p.JoinedAt
		if joinedAt.IsZero() {
			joinedAt = time.Now().UTC()
		}
		lastSeen := p.LastSeen
		if lastSeen.IsZero() {
			lastSeen = joinedAt
		}
		peers = append(peers, model.Peer{
			ID:          p.ID,
			RoomID:      room.ID,
			UserID:      p.UserID,
			DisplayName: p.DisplayName,
			Status:      string(status),
			JoinedAt:    joinedAt.UTC(),
			LastSeen:    lastSeen.UTC(),
		})
	}

	return &model.Room{
		ID:        room.ID,
		Name:      room.Name,
		Owner:     room.Owner,
		Link:      room.Link,
		CreatedAt: room.CreatedAt.UTC(),
		ExpiresAt: expiresAt,
		Peers:     peers,
	}
}

func toDomainRoom(room *model.Room) *domain.Room {
	peers := make(map[string]*domain.Peer, len(room.Peers))
	for i := range room.Peers {
		p := room.Peers[i]
		status := domain.PeerStatus(p.Status)
		if status == "" {
			status = domain.PeerStatusConnected
		}
		peer := &domain.Peer{
			ID:          p.ID,
			UserID:      p.UserID,
			DisplayName: p.DisplayName,
			Status:      status,
			JoinedAt:    p.JoinedAt.UTC(),
			LastSeen:    p.LastSeen.UTC(),
			Events:      make(chan domain.SignalMessage, 16),
		}
		peers[peer.ID] = peer
	}

	var expiresAt time.Time
	if room.ExpiresAt != nil {
		expiresAt = room.ExpiresAt.UTC()
	}

	return &domain.Room{
		ID:        room.ID,
		Name:      room.Name,
		Owner:     room.Owner,
		Link:      room.Link,
		Peers:     peers,
		CreatedAt: room.CreatedAt.UTC(),
		ExpiresAt: expiresAt,
	}
}

func toModelUser(user *domain.User) *model.User {
	var email *string
	if user.Email != "" {
		e := user.Email
		email = &e
	}
	return &model.User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     email,
		IsGuest:   user.IsGuest,
		CreatedAt: user.CreatedAt.UTC(),
		UpdatedAt: user.UpdatedAt.UTC(),
	}
}

func toDomainUser(user *model.User) *domain.User {
	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	return &domain.User{
		ID:        user.ID,
		Name:      user.Name,
		Email:     email,
		IsGuest:   user.IsGuest,
		CreatedAt: user.CreatedAt.UTC(),
		UpdatedAt: user.UpdatedAt.UTC(),
	}
}
