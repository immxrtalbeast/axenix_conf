package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
	"github.com/immxrtalbeast/axenix_conf/internal/repository"
)

type UserService struct {
	users repository.UserRepository
	log   *slog.Logger
}

func NewUserService(users repository.UserRepository, log *slog.Logger) *UserService {
	return &UserService{users: users, log: log}
}

func (s *UserService) CreateUser(ctx context.Context, name string, email string) (*domain.User, error) {
	const op = "service.user.create"
	s.log.With(
		slog.Attr(slog.String("op", op)),
	)
	s.log.Info("creating user")
	if name == "" {
		s.log.Error("no name provided")
		return nil, errors.New("name is required")
	}
	user := domain.NewUser(name, email)
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const op = "service.user.get"
	s.log.With(
		slog.Attr(slog.String("op", op)),
	)
	s.log.Info("getting user")
	return s.users.GetByID(ctx, id)
}

func (s *UserService) UpdateUser(ctx context.Context, user *domain.User) error {
	if user == nil {
		return errors.New("user is required")
	}
	user.UpdatedAt = time.Now().UTC()
	return s.users.Update(ctx, user)
}
