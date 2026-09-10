package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/amjil/mgl-push/mgl-push-server/internal/auth"
	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
	"github.com/amjil/mgl-push/mgl-push-server/internal/queue"
	"github.com/amjil/mgl-push/mgl-push-server/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	devices  *service.DeviceService
	messages *service.MessageService
	db       *pgxpool.Pool
	registry *provider.Registry
	worker   *queue.Worker
	mux      *http.ServeMux
}

func NewServer(
	devices *service.DeviceService,
	messages *service.MessageService,
	db *pgxpool.Pool,
	registry *provider.Registry,
	worker *queue.Worker,
) *Server {
	s := &Server{
		devices:  devices,
		messages: messages,
		db:       db,
		registry: registry,
		worker:   worker,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler(authenticator *auth.Authenticator) http.Handler {
	return authenticator.Middleware(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)
	s.mux.Handle("GET /metrics", promhttp.Handler())

	s.mux.HandleFunc("POST /v1/devices", s.handleRegisterDevice)
	s.mux.HandleFunc("PUT /v1/devices/{installation_id}/token", s.handleUpdateToken)
	s.mux.HandleFunc("PUT /v1/devices/{installation_id}/user", s.handleSetUser)
	s.mux.HandleFunc("DELETE /v1/devices/{installation_id}/user", s.handleClearUser)
	s.mux.HandleFunc("DELETE /v1/devices/{installation_id}", s.handleUnregister)
	s.mux.HandleFunc("POST /v1/messages", s.handleSendMessage)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := map[string]string{}
	ok := true

	if err := s.db.Ping(ctx); err != nil {
		checks["postgres"] = "unavailable"
		ok = false
	} else {
		checks["postgres"] = "ok"
	}

	if len(s.registry.Names()) == 0 {
		checks["providers"] = "none"
		ok = false
	} else {
		checks["providers"] = strings.Join(s.registry.Names(), ",")
	}

	if s.worker == nil || !s.worker.Alive() {
		checks["worker"] = "stopped"
		ok = false
	} else {
		checks["worker"] = "ok"
	}

	status := "ok"
	code := http.StatusOK
	if !ok {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": checks})
}

type registerDeviceRequest struct {
	InstallationID string `json:"installation_id"`
	UserID         string `json:"user_id"`
	Platform       string `json:"platform"`
	Provider       string `json:"provider"`
	Token          string `json:"token"`
	AppID          string `json:"app_id"`
	AppVersion     string `json:"app_version"`
	OSVersion      string `json:"os_version"`
	DeviceModel    string `json:"device_model"`
	Locale         string `json:"locale"`
	Timezone       string `json:"timezone"`
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var req registerDeviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, domain.InvalidRequest("invalid json"))
		return
	}
	if req.AppID == "" {
		req.AppID = appID
	}
	if req.AppID != appID {
		writeError(w, domain.Forbidden("app_id mismatch"))
		return
	}
	d, err := s.devices.Register(r.Context(), service.RegisterDeviceInput{
		InstallationID: req.InstallationID,
		UserID:         req.UserID,
		Platform:       req.Platform,
		Provider:       req.Provider,
		Token:          req.Token,
		AppID:          req.AppID,
		AppVersion:     req.AppVersion,
		OSVersion:      req.OSVersion,
		DeviceModel:    req.DeviceModel,
		Locale:         req.Locale,
		Timezone:       req.Timezone,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"device_id":       d.ID,
		"installation_id": d.InstallationID,
		"status":          d.Status,
	})
}

type updateTokenRequest struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
}

func (s *Server) handleUpdateToken(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	installationID := r.PathValue("installation_id")
	var req updateTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, domain.InvalidRequest("invalid json"))
		return
	}
	d, err := s.devices.UpdateToken(r.Context(), appID, installationID, req.Provider, req.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": d.Status})
}

type setUserRequest struct {
	UserID string `json:"user_id"`
}

func (s *Server) handleSetUser(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	installationID := r.PathValue("installation_id")
	var req setUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, domain.InvalidRequest("invalid json"))
		return
	}
	if err := s.devices.SetUserID(r.Context(), appID, installationID, req.UserID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleClearUser(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	installationID := r.PathValue("installation_id")
	if err := s.devices.ClearUserID(r.Context(), appID, installationID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleUnregister(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	installationID := r.PathValue("installation_id")
	if err := s.devices.Unregister(r.Context(), appID, installationID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

type sendMessageRequest struct {
	UserIDs         []string          `json:"user_ids"`
	InstallationIDs []string          `json:"installation_ids"`
	Provider        string            `json:"provider"`
	Token           string            `json:"token"`
	Notification    *notificationBody `json:"notification"`
	Data            map[string]string `json:"data"`
	ImageURL        string            `json:"image_url"`
	Priority        string            `json:"priority"`
	TTLSeconds      *int              `json:"ttl_seconds"`
	CollapseKey     string            `json:"collapse_key"`
	Sound           string            `json:"sound"`
	Badge           *int              `json:"badge"`
	DeepLink        string            `json:"deep_link"`
	Category        string            `json:"category"`
}

type notificationBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	appID, err := auth.RequireAppID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	var req sendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, domain.InvalidRequest("invalid json"))
		return
	}
	in := service.SendMessageInput{
		AppID:           appID,
		UserIDs:         req.UserIDs,
		InstallationIDs: req.InstallationIDs,
		Provider:        req.Provider,
		Token:           req.Token,
		Data:            req.Data,
		ImageURL:        req.ImageURL,
		Priority:        req.Priority,
		TTLSeconds:      req.TTLSeconds,
		CollapseKey:     req.CollapseKey,
		Sound:           req.Sound,
		Badge:           req.Badge,
		DeepLink:        req.DeepLink,
		Category:        req.Category,
	}
	if req.Notification != nil {
		in.Title = req.Notification.Title
		in.Body = req.Notification.Body
	}
	result, err := s.messages.Send(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"message_id": result.MessageID,
		"accepted":   result.Accepted,
	})
}

func decodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dest)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.HTTPStatus, map[string]any{
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"error": map[string]string{
			"code":    domain.ErrCodeInternalError,
			"message": "internal error",
		},
	})
}
