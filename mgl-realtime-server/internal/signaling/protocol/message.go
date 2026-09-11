package protocol

import (
	"encoding/json"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/idgen"
)

// Envelope is the unified WebSocket message format (spec §15).
type Envelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	CallID    string          `json:"call_id,omitempty"`
	SenderID  string          `json:"sender_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func New(typ string, callID, senderID string, data any) Envelope {
	var raw json.RawMessage
	if data != nil {
		switch v := data.(type) {
		case json.RawMessage:
			raw = v
		case []byte:
			raw = json.RawMessage(v)
		default:
			b, _ := json.Marshal(data)
			raw = b
		}
	}
	return Envelope{
		ID:        "msg_" + idgen.New(),
		Type:      typ,
		Timestamp: time.Now().UTC(),
		CallID:    callID,
		SenderID:  senderID,
		Data:      raw,
	}
}

func (e Envelope) DecodeData(dest any) error {
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, dest)
}

// Client → Server
const (
	TypeAuthenticate = "authenticate"
	TypePing         = "ping"
	TypePong         = "pong"

	TypeCallCreate = "call.create"
	TypeCallAccept = "call.accept"
	TypeCallReject = "call.reject"
	TypeCallCancel = "call.cancel"
	TypeCallJoin   = "call.join"
	TypeCallLeave  = "call.leave"
	TypeCallHangup = "call.hangup"
	TypeCallResume = "call.resume"

	TypeSessionResume = "session.resume"

	TypeWebRTCOffer        = "webrtc.offer"
	TypeWebRTCAnswer       = "webrtc.answer"
	TypeWebRTCICECandidate = "webrtc.ice_candidate"

	TypeMediaMute      = "media.mute"
	TypeMediaUnmute    = "media.unmute"
	TypeMediaCameraOn  = "media.camera_on"
	TypeMediaCameraOff = "media.camera_off"
)

// Server → Client
const (
	TypeAuthenticated = "authenticated"
	TypeError         = "error"

	TypeCallCreated           = "call.created"
	TypeCallRinging           = "call.ringing"
	TypeCallAccepted          = "call.accepted"
	TypeCallRejected          = "call.rejected"
	TypeCallCancelled         = "call.cancelled"
	TypeCallParticipantJoined = "call.participant_joined"
	TypeCallParticipantLeft   = "call.participant_left"
	TypeCallConnected         = "call.connected"
	TypeCallEnded             = "call.ended"
	TypeCallFailed            = "call.failed"
	TypeCallState             = "call.state"
	TypeCallStopRinging       = "call.stop_ringing"

	TypeSessionState    = "session.state"
	TypePresenceUpdated = "presence.updated"
)

type AuthenticateData struct {
	Token     string `json:"token"`
	DeviceID  string `json:"device_id,omitempty"`
	Reconnect bool   `json:"reconnect,omitempty"`
}

type AuthenticatedData struct {
	UserID      string `json:"user_id"`
	AppID       string `json:"app_id,omitempty"`
	DeviceID    string `json:"device_id,omitempty"`
	Reconnect   bool   `json:"reconnect,omitempty"`
	ActiveCalls any    `json:"active_calls,omitempty"`
}

type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StopRingingData struct {
	CallID          string `json:"call_id"`
	AcceptedDevice  string `json:"accepted_device_id,omitempty"`
	Reason          string `json:"reason,omitempty"`
}
