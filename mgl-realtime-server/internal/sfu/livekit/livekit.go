package livekit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/sfu"
)

// Provider issues HS256-style access tokens compatible with basic LiveKit JWT shape.
// Full LiveKit server SDK can replace this later without changing Call Service.
type Provider struct {
	url       string
	apiKey    string
	apiSecret string
}

func New(url, apiKey, apiSecret string) *Provider {
	return &Provider{url: url, apiKey: apiKey, apiSecret: apiSecret}
}

func (p *Provider) CreateRoom(context.Context, string) error { return nil }
func (p *Provider) DeleteRoom(context.Context, string) error { return nil }
func (p *Provider) URL() string                             { return p.url }

func (p *Provider) JoinToken(_ context.Context, roomID, userID, role string, ttl time.Duration) (string, error) {
	if p.apiKey == "" || p.apiSecret == "" {
		return "", fmt.Errorf("livekit credentials missing")
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now()
	video := map[string]any{
		"roomJoin": true,
		"room":     roomID,
	}
	if role == "publisher" || role == "caller" || role == "callee" || role == "" {
		video["canPublish"] = true
		video["canSubscribe"] = true
	} else if role == "subscriber" {
		video["canPublish"] = false
		video["canSubscribe"] = true
	}
	claims := map[string]any{
		"iss":   p.apiKey,
		"sub":   userID,
		"nbf":   now.Add(-time.Second).Unix(),
		"exp":   now.Add(ttl).Unix(),
		"video": video,
		"name":  userID,
	}
	return signJWT(claims, p.apiSecret)
}

var _ sfu.Provider = (*Provider)(nil)

func signJWT(claims map[string]any, secret string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}
