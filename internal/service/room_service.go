package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
	"github.com/immxrtalbeast/axenix_conf/internal/repository"
	"github.com/immxrtalbeast/axenix_conf/lib/logger/sl"
	"github.com/pion/webrtc/v3"
)

var (
	ErrRoomExpired  = errors.New("room expired")
	ErrPeerNotFound = errors.New("peer not found")
)

type RoomService struct {
	rooms        repository.RoomRepository
	users        repository.UserRepository
	webrtcConfig webrtc.Configuration
	log          *slog.Logger
	mu           sync.RWMutex
}

func NewRoomService(rooms repository.RoomRepository, users repository.UserRepository, config webrtc.Configuration, log *slog.Logger) *RoomService {
	if log == nil {
		log = slog.Default()
	}
	return &RoomService{
		rooms:        rooms,
		users:        users,
		webrtcConfig: config,
		log:          log,
	}
}

func (s *RoomService) CreateRoom(ctx context.Context, name string, owner uuid.UUID, lifetime time.Duration) (*domain.Room, error) {
	if name == "" {
		return nil, errors.New("room name is required")
	}
	if owner == uuid.Nil {
		return nil, errors.New("owner is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var room *domain.Room

	for {
		room = domain.NewRoom(name, owner, lifetime)
		s.log.Info("room created", room.ID)
		if err := s.rooms.Create(ctx, room); err != nil {
			if errors.Is(err, repository.ErrRoomLinkExists) {
				continue
			}
			return nil, err
		}
		break
	}

	return room, nil
}

func (s *RoomService) GetRoom(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	room, err := s.rooms.GetByID(ctx, id)
	s.log.Info("room retrieved",
		"room_id", room.ID,
		"name", room.Name,
		"owner", room.Owner,
		"link", room.Link,
		"created_at", room.CreatedAt,
		"expires_at", room.ExpiresAt,
		"peers_count", len(room.Peers),
	)
	if err != nil {
		return nil, err
	}

	if room.IsExpired() {
		return nil, ErrRoomExpired
	}
	return room, nil
}

func (s *RoomService) GetRoomByLink(ctx context.Context, link string) (*domain.Room, error) {
	room, err := s.rooms.GetByLink(ctx, link)
	if err != nil {
		return nil, err
	}

	if room.IsExpired() {
		return nil, ErrRoomExpired
	}
	return room, nil
}

func (s *RoomService) RegisterPeer(ctx context.Context, roomID uuid.UUID, user *domain.User) (*domain.Peer, error) {
	const op = "service.room.register.peer"
	log := s.log.With(
		slog.String("op", op),
		slog.String("room_id", roomID.String()),
	)

	if user == nil {
		return nil, errors.New("user is required")
	}

	room, err := s.GetRoom(ctx, roomID)

	log.Info("room info",
		"room_id", room.ID,
		"name", room.Name,
		"peers_count", len(room.Peers),
	)
	if err != nil {
		return nil, err
	}

	s.log.Info("room", room)

	if err := s.ensureUser(ctx, user); err != nil {
		return nil, err
	}

	peer := domain.NewPeer(user.ID, user.Name)

	conn, err := webrtc.NewPeerConnection(s.webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("create peer connection: %w", err)
	}

	peer.Connection = conn

	conn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			peer.SetStatus(domain.PeerStatusConnected)
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			peer.SetStatus(domain.PeerStatusDisconnected)
		default:
			peer.SetStatus(domain.PeerStatusConnecting)
		}
	})

	conn.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		select {
		case peer.Events <- domain.SignalMessage{
			Type:      "ice-candidate",
			Candidate: &init,
			Room:      room.ID.String(),
			SenderID:  peer.ID,
		}:
		default:
			s.log.Debug("dropping candidate event", slog.String("peer", peer.ID))
		}
	})

	conn.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		_ = receiver
		peer.Mutex.Lock()
		defer peer.Mutex.Unlock()
		peer.Streams[track.ID()] = track
	})

	room.Mutex.Lock()
	room.Peers[peer.ID] = peer
	room.Mutex.Unlock()

	if err := s.rooms.Update(ctx, room); err != nil {
		return nil, err
	}

	s.broadcast(room, domain.SignalMessage{
		Type:     "peer-joined",
		Room:     room.ID.String(),
		SenderID: peer.ID,
		Payload: map[string]any{
			"peer_id":      peer.ID,
			"user_id":      peer.UserID.String(),
			"display_name": peer.DisplayName,
		},
	}, peer.ID)
	log.Info("peer registered",
		"peer_id", peer.ID,
		"user_id", peer.UserID,
		"display_name", peer.DisplayName,
	)
	return peer, nil
}

func (s *RoomService) UnregisterPeer(ctx context.Context, roomID uuid.UUID, peerID string) error {
	s.log.Info("unregistering peer",
		"room_id", roomID.String(),
		"peer_id", peerID,
	)
	room, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}

	room.Mutex.Lock()
	defer room.Mutex.Unlock()

	peer, ok := room.Peers[peerID]
	if !ok {
		return ErrPeerNotFound
	}

	if peer.Connection != nil {
		_ = peer.Connection.Close()
	}
	delete(room.Peers, peerID)

	if err := s.rooms.Update(ctx, room); err != nil {
		s.log.Error("err", sl.Err(err))
		return err
	}

	s.broadcast(room, domain.SignalMessage{
		Type:     "peer-left",
		Room:     room.ID.String(),
		SenderID: peerID,
		Payload: map[string]any{
			"peer_id": peerID,
		},
	}, peerID)

	return nil
}

func (s *RoomService) HandleSignal(ctx context.Context, roomID uuid.UUID, peerID string, message *domain.SignalMessage) (*domain.SignalMessage, error) {
	const op = "service.room.signal"
	if message == nil {
		return nil, errors.New("message is required")
	}
	log := s.log.With(
		"op", op,
		"room_id", roomID.String(),
		"peer_id", peerID,
	)

	log.Info("new signal",
		"type", message.Type,
		"room", message.Room,
		"sender", message.SenderID)
	if message.Payload != nil {
		s.log.Debug("signal payload", "payload", message.Payload)
	}
	room, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	room.Mutex.RLock()
	peer, ok := room.Peers[peerID]
	room.Mutex.RUnlock()
	if !ok {
		return nil, ErrPeerNotFound
	}

	switch message.Type {
	case "offer":
		if message.SDP == nil {
			return nil, errors.New("offer missing SDP")
		}
		if err := peer.Connection.SetRemoteDescription(*message.SDP); err != nil {
			return nil, err
		}
		answer, err := peer.Connection.CreateAnswer(nil)
		if err != nil {
			return nil, err
		}
		if err := peer.Connection.SetLocalDescription(answer); err != nil {
			return nil, err
		}
		return &domain.SignalMessage{
			Type:     "answer",
			SDP:      &answer,
			SenderID: peer.ID,
			Room:     room.ID.String(),
		}, nil
	case "answer":
		if message.SDP == nil {
			return nil, errors.New("answer missing SDP")
		}
		if err := peer.Connection.SetRemoteDescription(*message.SDP); err != nil {
			return nil, err
		}
	case "ice-candidate":
		if message.Candidate == nil {
			return nil, errors.New("candidate missing payload")
		}
		if err := peer.Connection.AddICECandidate(*message.Candidate); err != nil {
			return nil, err
		}
	case "leave":
		return nil, s.UnregisterPeer(ctx, roomID, peerID)
	default:
		return nil, fmt.Errorf("unsupported signal type: %s", message.Type)
	}

	return nil, nil
}

func (s *RoomService) ListParticipants(ctx context.Context, roomID uuid.UUID) ([]*domain.User, error) {
	room, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	users := make([]*domain.User, 0, len(room.Peers))

	room.Mutex.RLock()
	defer room.Mutex.RUnlock()

	for _, peer := range room.Peers {
		user, err := s.users.GetByID(ctx, peer.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				continue
			}
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (s *RoomService) ensureUser(ctx context.Context, user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	user.UpdatedAt = time.Now().UTC()

	_, err := s.users.GetByID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return s.users.Create(ctx, user)
		}
		return err
	}

	return s.users.Update(ctx, user)
}

func (s *RoomService) broadcast(room *domain.Room, msg domain.SignalMessage, exclude string) {
	room.Mutex.RLock()
	peers := make([]*domain.Peer, 0, len(room.Peers))
	for id, peer := range room.Peers {
		if id == exclude {
			continue
		}
		peers = append(peers, peer)
	}
	room.Mutex.RUnlock()

	for _, peer := range peers {
		select {
		case peer.Events <- msg:
		default:
			s.log.Debug("dropping broadcast event", slog.String("peer", peer.ID), slog.String("type", msg.Type))
		}
	}
}
