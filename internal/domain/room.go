package domain

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

const linkLength = 12

// Room represents a WebRTC session that can host multiple peers.
// It stores the metadata required for validation and quick signalling lookups.
type Room struct {
	Mutex     sync.RWMutex
	ID        uuid.UUID
	Name      string
	Owner     uuid.UUID
	Link      string
	Peers     map[string]*Peer
	Tracks    map[string]*webrtc.TrackLocalStaticRTP
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewRoom constructs a room with generated identifiers and lifetime options.
func NewRoom(name string, owner uuid.UUID, lifetime time.Duration) *Room {
	id := uuid.New()
	now := time.Now().UTC()
	room := &Room{
		ID:        id,
		Name:      name,
		Owner:     owner,
		Link:      generateLink(),
		Peers:     make(map[string]*Peer),
		Tracks:    make(map[string]*webrtc.TrackLocalStaticRTP),
		CreatedAt: now,
	}

	if lifetime > 0 {
		room.ExpiresAt = now.Add(lifetime)
	}

	return room
}

// IsExpired reports whether the room is no longer valid.
func (r *Room) IsExpired() bool {
	if r == nil {
		return true
	}
	if r.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().UTC().After(r.ExpiresAt)
}

func generateLink() string {
	link := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(link) <= linkLength {
		return link
	}
	return link[:linkLength]
}
