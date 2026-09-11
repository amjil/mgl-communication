package sfu

import (
	"context"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/config"
)

// RoomInfo describes an SFU room binding for a call.
type RoomInfo struct {
	RoomID    string `json:"room_id"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	Transport string `json:"transport"` // p2p | sfu
}

type ICEConfig struct {
	ICEServers []config.ICEServer `json:"ice_servers"`
}

// Provider creates rooms / tokens. LiveKit (or stub) implements this.
type Provider interface {
	CreateRoom(ctx context.Context, roomID string) error
	DeleteRoom(ctx context.Context, roomID string) error
	JoinToken(ctx context.Context, roomID, userID, role string, ttl time.Duration) (string, error)
	URL() string
}

// StubProvider is used when LiveKit credentials are not configured.
type StubProvider struct {
	url string
}

func NewStub(url string) *StubProvider {
	if url == "" {
		url = "sfu://stub"
	}
	return &StubProvider{url: url}
}

func (s *StubProvider) CreateRoom(context.Context, string) error { return nil }
func (s *StubProvider) DeleteRoom(context.Context, string) error { return nil }
func (s *StubProvider) JoinToken(_ context.Context, roomID, userID, role string, _ time.Duration) (string, error) {
	return "stub." + roomID + "." + userID + "." + role, nil
}
func (s *StubProvider) URL() string { return s.url }

type Service struct {
	provider   Provider
	iceServers []config.ICEServer
}

func NewService(provider Provider, ice []config.ICEServer) *Service {
	return &Service{provider: provider, iceServers: ice}
}

func (s *Service) ICEConfig() ICEConfig {
	return ICEConfig{ICEServers: s.iceServers}
}

func (s *Service) EnsureRoom(ctx context.Context, roomID string) error {
	return s.provider.CreateRoom(ctx, roomID)
}

func (s *Service) Join(ctx context.Context, roomID, userID, role string, ttl time.Duration) (*RoomInfo, error) {
	if err := s.provider.CreateRoom(ctx, roomID); err != nil {
		return nil, err
	}
	tok, err := s.provider.JoinToken(ctx, roomID, userID, role, ttl)
	if err != nil {
		return nil, err
	}
	return &RoomInfo{
		RoomID:    roomID,
		URL:       s.provider.URL(),
		Token:     tok,
		Transport: "sfu",
	}, nil
}

func (s *Service) P2PRoom(roomID string) *RoomInfo {
	return &RoomInfo{
		RoomID:    roomID,
		Transport: "p2p",
	}
}
