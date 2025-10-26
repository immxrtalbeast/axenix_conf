package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type PeerStatus string

const (
	PeerStatusConnected    PeerStatus = "connected"
	PeerStatusConnecting   PeerStatus = "connecting"
	PeerStatusDisconnected PeerStatus = "disconnected"
)

// Peer represents an active participant in the room.
type Peer struct {
	ID          string
	UserID      uuid.UUID
	DisplayName string
	Status      PeerStatus
	JoinedAt    time.Time
	LastSeen    time.Time
	Mutex       sync.RWMutex
	Socket      *websocket.Conn
	Events      chan SignalMessage
}

func NewPeer(userID uuid.UUID, displayName string) *Peer {
	return &Peer{
		ID:          uuid.New().String(),
		UserID:      userID,
		DisplayName: displayName,
		Status:      PeerStatusConnecting,
		JoinedAt:    time.Now().UTC(),
		LastSeen:    time.Now().UTC(),
		Events:      make(chan SignalMessage, 16),
	}
}

func (p *Peer) Touch() {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.LastSeen = time.Now().UTC()
}

func (p *Peer) EnqueueEvent(event SignalMessage) {
	select {
	case p.Events <- event:
	default:
	}
}

func (p *Peer) SetStatus(status PeerStatus) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.Status = status
}
