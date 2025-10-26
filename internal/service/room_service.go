package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
	"github.com/immxrtalbeast/axenix_conf/internal/repository"
	"github.com/immxrtalbeast/axenix_conf/lib/logger/sl"
)

var (
	ErrRoomExpired  = errors.New("room expired")
	ErrPeerNotFound = errors.New("peer not found")
)

const maxChatMessageLength = 4000
const maxChatSenderLength = 255

type chatPayloadData struct {
	message   string
	sender    string
	id        uuid.UUID
	timestamp time.Time
}

type RoomService struct {
	rooms       repository.RoomRepository
	users       repository.UserRepository
	log         *slog.Logger
	mu          sync.RWMutex
	activeRooms map[uuid.UUID]*domain.Room
}

func NewRoomService(rooms repository.RoomRepository, users repository.UserRepository, log *slog.Logger) *RoomService {
	if log == nil {
		log = slog.Default()
	}
	return &RoomService{
		rooms:       rooms,
		users:       users,
		log:         log,
		activeRooms: make(map[uuid.UUID]*domain.Room),
	}
}

func (s *RoomService) CreateRoom(ctx context.Context, name string, owner uuid.UUID, lifetime time.Duration) (*domain.Room, error) {
	if name == "" {
		return nil, errors.New("room name is required")
	}
	if owner == uuid.Nil {
		return nil, errors.New("owner is required")
	}

	for {
		room := domain.NewRoom(name, owner, lifetime)
		s.log.Info("room created", "room_id", room.ID.String())
		if err := s.rooms.Create(ctx, room); err != nil {
			if errors.Is(err, repository.ErrRoomLinkExists) {
				continue
			}
			return nil, err
		}

		s.mu.Lock()
		s.activeRooms[room.ID] = room
		s.mu.Unlock()

		return room, nil
	}
}

func (s *RoomService) GetRoom(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	if room := s.getActiveRoom(id); room != nil {
		if room.IsExpired() {
			s.removeActiveRoom(id)
			return nil, ErrRoomExpired
		}
		s.logRoom(room)
		return room, nil
	}

	roomFromDB, err := s.rooms.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	room := s.activateRoom(roomFromDB)
	if room.IsExpired() {
		s.removeActiveRoom(room.ID)
		return nil, ErrRoomExpired
	}

	s.logRoom(room)
	return room, nil
}

func (s *RoomService) GetRoomByLink(ctx context.Context, link string) (*domain.Room, error) {
	if room := s.getActiveRoomByLink(link); room != nil {
		if room.IsExpired() {
			s.removeActiveRoom(room.ID)
			return nil, ErrRoomExpired
		}
		return room, nil
	}

	roomFromDB, err := s.rooms.GetByLink(ctx, link)
	if err != nil {
		return nil, err
	}

	room := s.activateRoom(roomFromDB)
	if room.IsExpired() {
		s.removeActiveRoom(room.ID)
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
	if err != nil {
		log.Info("err", sl.Err(err))
		return nil, err
	}

	log.Info("room info",
		"room_id", room.ID,
		"name", room.Name,
		"peers_count", len(room.Peers),
	)

	if err := s.ensureUser(ctx, user); err != nil {
		log.Info("ensure user failed", "details", sl.Err(err))
		return nil, err
	}

	peer := domain.NewPeer(user.ID, user.Name)

	existingPeers := make([]*domain.Peer, 0, len(room.Peers))
	room.Mutex.RLock()
	for _, p := range room.Peers {
		existingPeers = append(existingPeers, p)
	}
	room.Mutex.RUnlock()

	room.Mutex.Lock()
	room.Peers[peer.ID] = peer
	room.Mutex.Unlock()

	if err := s.rooms.Update(ctx, room); err != nil {
		log.Info("failed to update", "details", sl.Err(err))
		return nil, err
	}

	for _, existing := range existingPeers {
		peer.EnqueueEvent(domain.SignalMessage{
			Type:     "joined",
			Room:     room.ID.String(),
			SenderID: existing.ID,
			Payload: map[string]any{
				"peer_id":      existing.ID,
				"user_id":      existing.UserID.String(),
				"display_name": existing.DisplayName,
			},
		})
	}

	s.broadcast(room, domain.SignalMessage{
		Type:     "joined",
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
	peer, ok := room.Peers[peerID]
	if !ok {
		room.Mutex.Unlock()
		return ErrPeerNotFound
	}

	delete(room.Peers, peerID)
	roomEmpty := len(room.Peers) == 0
	room.Mutex.Unlock()

	if peer != nil {
		peer.SetStatus(domain.PeerStatusDisconnected)
		if peer.Events != nil {
			close(peer.Events)
		}
		if peer.Socket != nil {
			peer.Socket.Close()
			peer.Socket = nil
		}
	}

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

	if roomEmpty {
		s.removeActiveRoom(room.ID)
	}

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
	case "offer", "answer", "ice-candidate":
		forward := *message
		forward.Room = room.ID.String()
		forward.SenderID = peer.ID

		if forward.TargetID == "" {
			s.broadcast(room, forward, peer.ID)
			return nil, nil
		}
		room.Mutex.RLock()
		targetPeer, ok := room.Peers[forward.TargetID]
		room.Mutex.RUnlock()
		if !ok {
			return nil, ErrPeerNotFound
		}
		targetPeer.EnqueueEvent(forward)
	case "chat":
		payloadData, err := validateChatPayload(message.Payload)
		if err != nil {
			return nil, err
		}

		chatMsg := domain.NewChatMessage(room.ID, peer, payloadData.message)
		if payloadData.id != uuid.Nil {
			chatMsg.ID = payloadData.id
		}
		if payloadData.sender != "" {
			chatMsg.DisplayName = payloadData.sender
		}
		if !payloadData.timestamp.IsZero() {
			chatMsg.CreatedAt = payloadData.timestamp.UTC()
		}

		if err := s.rooms.SaveChatMessage(ctx, chatMsg); err != nil {
			log.Error("failed to save chat message", sl.Err(err))
			return nil, err
		}

		event := domain.SignalMessage{
			Type:     "chat",
			Room:     room.ID.String(),
			SenderID: peer.ID,
			Payload: map[string]any{
				"id":        chatMsg.ID.String(),
				"sender":    chatMsg.DisplayName,
				"message":   chatMsg.Content,
				"timestamp": chatMsg.CreatedAt.UTC().Format(time.RFC3339Nano),
			},
		}

		s.broadcastAll(room, event)
	case "leave":
		log.Info("type is leaving")
		return nil, s.UnregisterPeer(ctx, roomID, peerID)
	default:
		return nil, errors.New("unsupported signal type: " + message.Type)
	}

	return nil, nil
}

func (s *RoomService) ListParticipants(ctx context.Context, roomID uuid.UUID) ([]*domain.User, error) {
	const op = "service.room.listParticipants"

	log := s.log.With(
		"op", op,
		"room_id", roomID.String(),
	)
	log.Info("trying to get room")
	room, err := s.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	log.Info("room getted")
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
	log.Info("participants getted successufully")
	return users, nil
}

func (s *RoomService) ensureUser(ctx context.Context, user *domain.User) error {
	const op = "service.room.ensureUser"
	log := s.log.With(slog.String("op", op), slog.String("user_id", user.ID.String()))

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now().UTC()
	}
	user.UpdatedAt = time.Now().UTC()

	log.Info("checking if user exists")

	_, err := s.users.GetByID(ctx, user.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			log.Info("user not found, creating new one")
			return s.users.Create(ctx, user)
		}
		log.Error("error getting user", "error", err)
		return err
	}
	log.Info("user exists, updating")

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

func (s *RoomService) broadcastAll(room *domain.Room, msg domain.SignalMessage) {
	room.Mutex.RLock()
	peers := make([]*domain.Peer, 0, len(room.Peers))
	for _, peer := range room.Peers {
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

func (s *RoomService) getActiveRoom(id uuid.UUID) *domain.Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeRooms[id]
}

func (s *RoomService) getActiveRoomByLink(link string) *domain.Room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, room := range s.activeRooms {
		if room.Link == link {
			return room
		}
	}
	return nil
}

func (s *RoomService) removeActiveRoom(id uuid.UUID) {
	s.mu.Lock()
	delete(s.activeRooms, id)
	s.mu.Unlock()
}

func (s *RoomService) activateRoom(room *domain.Room) *domain.Room {
	if room == nil {
		return nil
	}

	if room.Peers == nil {
		room.Peers = make(map[string]*domain.Peer)
	} else {
		for _, peer := range room.Peers {
			if peer == nil {
				continue
			}
			peer.Events = make(chan domain.SignalMessage, 16)
			if peer.Status == "" {
				peer.Status = domain.PeerStatusDisconnected
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing := s.activeRooms[room.ID]; existing != nil {
		return existing
	}

	s.activeRooms[room.ID] = room
	return room
}

func (s *RoomService) logRoom(room *domain.Room) {
	s.log.Info("room retrieved",
		"room_id", room.ID,
		"name", room.Name,
		"owner", room.Owner,
		"link", room.Link,
		"created_at", room.CreatedAt,
		"expires_at", room.ExpiresAt,
		"peers_count", len(room.Peers),
	)
}

func validateChatPayload(payload map[string]any) (*chatPayloadData, error) {
	if payload == nil {
		return nil, errors.New("chat payload is required")
	}

	rawMessage, ok := payload["message"]
	if !ok {
		return nil, errors.New("chat payload message is required")
	}

	message, ok := rawMessage.(string)
	if !ok {
		return nil, errors.New("chat payload message must be string")
	}

	trimmedMsg := strings.TrimSpace(message)
	if trimmedMsg == "" {
		return nil, errors.New("chat message cannot be empty")
	}

	if utf8.RuneCountInString(trimmedMsg) > maxChatMessageLength {
		return nil, errors.New("chat message is too long")
	}

	result := &chatPayloadData{
		message: trimmedMsg,
	}

	if rawSender, ok := payload["sender"]; ok && rawSender != nil {
		senderStr, ok := rawSender.(string)
		if !ok {
			return nil, errors.New("chat payload sender must be string")
		}
		trimmedSender := strings.TrimSpace(senderStr)
		if utf8.RuneCountInString(trimmedSender) > maxChatSenderLength {
			return nil, errors.New("chat sender is too long")
		}
		result.sender = trimmedSender
	}

	if rawID, ok := payload["id"]; ok && rawID != nil {
		idStr, ok := rawID.(string)
		if !ok {
			return nil, errors.New("chat payload id must be string")
		}
		idStr = strings.TrimSpace(idStr)
		if idStr != "" {
			parsed, err := uuid.Parse(idStr)
			if err != nil {
				return nil, errors.New("chat payload id must be valid uuid")
			}
			result.id = parsed
		}
	}

	if rawTimestamp, ok := payload["timestamp"]; ok && rawTimestamp != nil {
		tsStr, ok := rawTimestamp.(string)
		if !ok {
			return nil, errors.New("chat payload timestamp must be string")
		}
		tsStr = strings.TrimSpace(tsStr)
		if tsStr != "" {
			parsed, err := time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				parsed, err = time.Parse(time.RFC3339, tsStr)
				if err != nil {
					return nil, errors.New("chat payload timestamp must be RFC3339 formatted")
				}
			}
			result.timestamp = parsed.UTC()
		}
	}

	return result, nil
}
