package api

import (
	"net/http"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/metrics"
)

type createCallRequest struct {
	CallerID     string   `json:"caller_id"`
	CallerDevice string   `json:"device_id"`
	CalleeID     string   `json:"callee_id"`
	CalleeIDs    []string `json:"callee_ids"`
	Type         string   `json:"type"`
	Mode         string   `json:"mode"`
	CallerName   string   `json:"caller_name"`
}

type callActorRequest struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
}

func (s *Server) handleCreateCall(w http.ResponseWriter, r *http.Request) {
	if s.calls == nil {
		writeError(w, domain.Internal("call service unavailable"))
		return
	}
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var req createCallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, domain.InvalidRequest("invalid json"))
		return
	}
	callerID, err := auth.RequireUserID(r.Context(), req.CallerID)
	if err != nil {
		writeError(w, err)
		return
	}
	deviceID := auth.DeviceIDFromContext(r.Context())
	if req.CallerDevice != "" {
		deviceID = req.CallerDevice
	}
	callees := req.CalleeIDs
	if len(callees) == 0 && req.CalleeID != "" {
		callees = []string{req.CalleeID}
	}
	created, err := s.calls.CreateCtx(r.Context(), call.CreateInput{
		AppID:        appID,
		CallerID:     callerID,
		CallerDevice: deviceID,
		CalleeIDs:    callees,
		Type:         req.Type,
		Mode:         req.Mode,
		CallerName:   req.CallerName,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	metrics.CallsCreatedTotal.WithLabelValues(appID).Inc()
	if s.incoming != nil {
		go s.incoming.NotifyRinging(r.Context(), created, req.CallerName)
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetCall(w http.ResponseWriter, r *http.Request) {
	if s.calls == nil {
		writeError(w, domain.Internal("call service unavailable"))
		return
	}
	c, err := s.calls.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleResumeCall(w http.ResponseWriter, r *http.Request) {
	if s.calls == nil {
		writeError(w, domain.Internal("call service unavailable"))
		return
	}
	var req callActorRequest
	_ = decodeJSON(r, &req)
	userID, err := auth.RequireUserID(r.Context(), req.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	deviceID := auth.DeviceIDFromContext(r.Context())
	if req.DeviceID != "" {
		deviceID = req.DeviceID
	}
	snapshot, err := s.calls.ResumeFull(r.Context(), r.PathValue("id"), userID, deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAcceptCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, deviceID string) (*call.Call, error) {
		c, err := s.calls.Accept(callID, userID, deviceID)
		if err != nil {
			return nil, err
		}
		if s.incoming != nil {
			go s.incoming.NotifyStopRinging(r.Context(), c, userID, "accepted_elsewhere")
		}
		return c, nil
	})
}

func (s *Server) handleRejectCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, _ string) (*call.Call, error) {
		return s.calls.Reject(callID, userID)
	})
}

func (s *Server) handleCancelCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, _ string) (*call.Call, error) {
		c, err := s.calls.Cancel(callID, userID)
		if err == nil && s.incoming != nil {
			go s.incoming.NotifyCancelled(r.Context(), c, "cancelled")
		}
		return c, err
	})
}

func (s *Server) handleJoinCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, deviceID string) (*call.Call, error) {
		return s.calls.Join(callID, userID, deviceID)
	})
}

func (s *Server) handleLeaveCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, _ string) (*call.Call, error) {
		return s.calls.Leave(callID, userID)
	})
}

func (s *Server) handleHangupCall(w http.ResponseWriter, r *http.Request) {
	s.withActor(w, r, func(callID, userID, _ string) (*call.Call, error) {
		return s.calls.Hangup(callID, userID)
	})
}

func (s *Server) handleCallToken(w http.ResponseWriter, r *http.Request) {
	if s.calls == nil {
		writeError(w, domain.Internal("call service unavailable"))
		return
	}
	var req callActorRequest
	_ = decodeJSON(r, &req)
	userID, err := auth.RequireUserID(r.Context(), req.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	tok, err := s.calls.IssueToken(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tok)
}

func (s *Server) handleGetPresence(w http.ResponseWriter, r *http.Request) {
	if s.presence == nil {
		writeError(w, domain.Internal("presence unavailable"))
		return
	}
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	userID := r.PathValue("user_id")
	writeJSON(w, http.StatusOK, s.presence.GetUser(appID, userID))
}

func (s *Server) withActor(w http.ResponseWriter, r *http.Request, fn func(callID, userID, deviceID string) (*call.Call, error)) {
	if s.calls == nil {
		writeError(w, domain.Internal("call service unavailable"))
		return
	}
	var req callActorRequest
	_ = decodeJSON(r, &req)
	userID, err := auth.RequireUserID(r.Context(), req.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	deviceID := auth.DeviceIDFromContext(r.Context())
	if req.DeviceID != "" {
		deviceID = req.DeviceID
	}
	c, err := fn(r.PathValue("id"), userID, deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
