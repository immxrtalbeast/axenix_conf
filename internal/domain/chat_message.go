package domain

import (
	"time"

	"github.com/google/uuid"
)

type ChatMessage struct {
	ID          uuid.UUID
	RoomID      uuid.UUID
	UserID      uuid.UUID
	PeerID      string
	DisplayName string
	Content     string
	CreatedAt   time.Time
}

func NewChatMessage(roomID uuid.UUID, peer *Peer, content string) *ChatMessage {
	msg := &ChatMessage{
		ID:        uuid.New(),
		RoomID:    roomID,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	if peer != nil {
		msg.UserID = peer.UserID
		msg.PeerID = peer.ID
		msg.DisplayName = peer.DisplayName
	}
	return msg
}
