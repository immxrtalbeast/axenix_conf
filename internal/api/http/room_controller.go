package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/immxrtalbeast/axenix_conf/internal/api/http/converter"
	"github.com/immxrtalbeast/axenix_conf/internal/domain"
	"github.com/immxrtalbeast/axenix_conf/internal/service"
	"github.com/pion/webrtc/v3"
)

type RoomController struct {
	rooms    service.RoomInteractor
	users    service.UserInteractor
	upgrader websocket.Upgrader
}

func NewRoomController(rooms service.RoomInteractor, users service.UserInteractor) *RoomController {
	return &RoomController{
		rooms: rooms,
		users: users,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (c *RoomController) CreateRoom(ctx *gin.Context) {
	type CreateRoomRequest struct {
		Name           string `json:"name" binding:"required"`
		Owner          string `json:"owner" binding:"required"`
		LifetimeMinute int    `json:"lifetime_minutes"`
	}
	var req CreateRoomRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	owner, err := uuid.Parse(req.Owner)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid owner uuid", "details": err.Error()})
		return
	}
	lifetime := time.Duration(req.LifetimeMinute) * time.Minute
	room, err := c.rooms.CreateRoom(ctx.Request.Context(), req.Name, owner, lifetime)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"room": converter.RoomToApi(room)})
}

func (c *RoomController) GetRoom(ctx *gin.Context) {
	roomID, err := uuid.Parse(ctx.Param("roomID"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	room, err := c.rooms.GetRoom(ctx.Request.Context(), roomID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrRoomExpired) {
			status = http.StatusGone
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"room": converter.RoomToApi(room)})
}

func (c *RoomController) GetRoomByLink(ctx *gin.Context) {
	room, err := c.rooms.GetRoomByLink(ctx.Request.Context(), ctx.Param("link"))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrRoomExpired) {
			status = http.StatusGone
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"room": converter.RoomToApi(room)})
}

func (c *RoomController) ListParticipants(ctx *gin.Context) {
	roomID, err := uuid.Parse(ctx.Param("roomID"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	users, err := c.rooms.ListParticipants(ctx.Request.Context(), roomID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"participants": users})
}
func (c *RoomController) JoinRoom(ctx *gin.Context) {
	roomID, err := uuid.Parse(ctx.Param("roomID"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	displayName := ctx.Query("name")
	if displayName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var user *domain.User
	if userIDStr := ctx.Query("user_id"); userIDStr != "" {
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}
		u, err := c.users.GetUser(ctx.Request.Context(), userID)
		if err != nil {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		user = u
		user.Name = displayName
	} else {
		user = domain.NewGuestUser(displayName)
	}

	conn, err := c.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "failed to upgrade connection", gin.H{"details": err})
		return
	}

	peer, err := c.rooms.RegisterPeer(context.Background(), roomID, user)
	if err != nil {
		conn.WriteJSON(gin.H{"error": err.Error()})
		conn.Close()
		return
	}
	peer.Socket = conn
	peer.SetStatus(domain.PeerStatusConnected)

	go forwardPeerEvents(peer)

	_ = conn.WriteJSON(domain.SignalMessage{
		Type:     "joined",
		Room:     roomID.String(),
		SenderID: peer.ID,
		Payload: map[string]any{
			"user_id":      peer.UserID.String(),
			"display_name": peer.DisplayName,
		},
	})

	for {
		var msg domain.SignalMessage
		if err := conn.ReadJSON(&msg); err != nil {
			_ = c.rooms.UnregisterPeer(context.Background(), roomID, peer.ID)
			conn.Close()
			return
		}

		resp, err := c.rooms.HandleSignal(context.Background(), roomID, peer.ID, &msg)
		if err != nil {
			conn.WriteJSON(gin.H{"error": err.Error()})
			continue
		}

		if resp != nil {
			if resp.SDP != nil {
				resp.SDP.Type = webrtc.SDPTypeAnswer
			}
			if err := conn.WriteJSON(resp); err != nil {
				_ = c.rooms.UnregisterPeer(context.Background(), roomID, peer.ID)
				conn.Close()
				return
			}
		}
	}
}

func forwardPeerEvents(peer *domain.Peer) {
	for event := range peer.Events {
		if peer.Socket == nil {
			return
		}
		if err := peer.Socket.WriteJSON(event); err != nil {
			return
		}
	}
}
