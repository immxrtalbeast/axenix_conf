package converter

import (
	"time"

	"github.com/google/uuid"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
)

type RoomResponse struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Owner     uuid.UUID      `json:"owner"`
	Link      string         `json:"link"`
	Peers     []PeerResponse `json:"peers"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
	IsExpired bool           `json:"is_expired"`
}

type PeerResponse struct {
	ID          string            `json:"id"`
	UserID      uuid.UUID         `json:"user_id"`
	DisplayName string            `json:"display_name"`
	Status      domain.PeerStatus `json:"status"`
}

func RoomToApi(r *domain.Room) *RoomResponse {
	peers := make([]PeerResponse, 0, len(r.Peers))

	r.Mutex.RLock()
	for _, peer := range r.Peers {
		peers = append(peers, PeerResponse{
			ID:          peer.ID,
			UserID:      peer.UserID,
			DisplayName: peer.DisplayName,
			Status:      peer.Status,
		})
	}
	r.Mutex.RUnlock()

	return &RoomResponse{
		ID:        r.ID,
		Name:      r.Name,
		Owner:     r.Owner,
		Link:      r.Link,
		Peers:     peers,
		CreatedAt: r.CreatedAt,
		ExpiresAt: r.ExpiresAt,
		IsExpired: r.IsExpired(),
	}
}
