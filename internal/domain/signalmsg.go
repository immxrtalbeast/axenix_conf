package domain

import "github.com/pion/webrtc/v3"

type SignalMessage struct {
	Type      string                     `json:"type"` // "offer", "answer", "ice-candidate", "join", "leave"
	SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
	Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
	Room      string                     `json:"room,omitempty"`
	SenderID  string                     `json:"sender_id,omitempty"`
	TargetID  string                     `json:"target_id,omitempty"`
	Payload   map[string]any             `json:"payload,omitempty"`
}
