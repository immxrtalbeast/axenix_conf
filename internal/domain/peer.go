package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
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
	Connection  *webrtc.PeerConnection
	Streams     map[string]*webrtc.TrackRemote
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
		Streams:     make(map[string]*webrtc.TrackRemote),
		Events:      make(chan SignalMessage, 16),
	}
}

func (p *Peer) Touch() {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.LastSeen = time.Now().UTC()
}

func (p *Peer) SetStatus(status PeerStatus) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.Status = status
}

// type PeerInterface interface {
// 	SetSocket(ws_conn *websocket.Conn)
// 	AddRemoteTrack(track *webrtc.TrackRemote)
// 	RemoveRemoteTrack(track *webrtc.TrackRemote)
// 	SetPeerConnection(conn *webrtc.PeerConnection)
// 	ReactOnOffer(offer webrtc.SessionDescription)
// 	ReactOnAnswer(answer_str string)
// }
