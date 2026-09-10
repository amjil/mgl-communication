package apns

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/net/http2"
)

const (
	hostProduction = "https://api.push.apple.com"
	hostSandbox    = "https://api.sandbox.push.apple.com"
	jwtTTL         = 50 * time.Minute
)

type Config struct {
	TeamID     string
	KeyID      string
	BundleID   string
	PrivateKey string // PEM contents or path to .p8
	Production bool
}

type Provider struct {
	cfg        Config
	httpClient *http.Client
	host       string

	mu        sync.Mutex
	jwt       string
	jwtExpiry time.Time
	key       *ecdsa.PrivateKey
}

func New(cfg Config) (*Provider, error) {
	if cfg.TeamID == "" || cfg.KeyID == "" || cfg.BundleID == "" || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("apns: team_id, key_id, bundle_id and private_key are required")
	}
	pemBytes, err := loadPrivateKeyPEM(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	key, err := parseECPrivateKey(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("apns: parse private key: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2: true,
	}
	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, fmt.Errorf("apns: http2: %w", err)
	}

	host := hostSandbox
	if cfg.Production {
		host = hostProduction
	}

	return &Provider{
		cfg:  cfg,
		key:  key,
		host: host,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}, nil
}

func (p *Provider) Name() string { return domain.ProviderAPNs }

func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	t := strings.TrimSpace(token)
	if t == "" || len(t) < 16 {
		return &provider.ProviderError{
			Provider:     domain.ProviderAPNs,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "invalid apns device token",
			InvalidToken: true,
		}
	}
	return nil
}

func (p *Provider) Send(ctx context.Context, message *domain.Message, device *domain.Device) (*provider.SendResult, error) {
	if err := p.ValidateToken(ctx, device.Token); err != nil {
		return &provider.SendResult{
			Accepted:     false,
			InvalidToken: true,
			ErrorCode:    domain.ErrCodeInvalidToken,
			ErrorMessage: err.Error(),
		}, nil
	}

	payload, err := buildPayload(message)
	if err != nil {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			ErrorCode:    domain.ErrCodeInvalidRequest,
			ErrorMessage: err.Error(),
		}, nil
	}

	token, err := p.bearerToken()
	if err != nil {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    true,
			ErrorCode:    domain.ErrCodeProviderAuthError,
			ErrorMessage: err.Error(),
		}, nil
	}

	url := fmt.Sprintf("%s/3/device/%s", p.host, device.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "bearer "+token)
	req.Header.Set("apns-topic", p.cfg.BundleID)
	req.Header.Set("apns-push-type", pushType(message))
	req.Header.Set("apns-priority", apnsPriority(message.Priority))
	if message.CollapseKey != "" {
		req.Header.Set("apns-collapse-id", truncate(message.CollapseKey, 64))
	}
	if message.ID != "" {
		req.Header.Set("apns-id", normalizeAPNsID(message.ID))
	}
	if message.TTL > 0 {
		exp := time.Now().Add(message.TTL).Unix()
		req.Header.Set("apns-expiration", fmt.Sprintf("%d", exp))
	}

	res, err := p.httpClient.Do(req)
	if err != nil {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    true,
			ErrorCode:    domain.ErrCodeProviderTempError,
			ErrorMessage: err.Error(),
		}, nil
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))

	if res.StatusCode == http.StatusOK {
		apnsID := res.Header.Get("apns-id")
		return &provider.SendResult{
			Accepted:          true,
			ProviderMessageID: apnsID,
		}, nil
	}
	return mapAPNsError(res.StatusCode, body), nil
}

type apnsErrorBody struct {
	Reason    string `json:"reason"`
	Timestamp int64  `json:"timestamp"`
}

func mapAPNsError(status int, body []byte) *provider.SendResult {
	var eb apnsErrorBody
	_ = json.Unmarshal(body, &eb)
	reason := strings.ToLower(eb.Reason)
	msg := string(body)
	if eb.Reason != "" {
		msg = eb.Reason
	}

	invalidReasons := map[string]bool{
		"baddevicetoken": true, "unregistered": true, "devicetokennotfortopic": true,
		"topicdisallowed": false,
	}
	if invalidReasons[reason] || reason == "unregistered" || reason == "baddevicetoken" {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			InvalidToken: true,
			ErrorCode:    domain.ErrCodeInvalidToken,
			ErrorMessage: msg,
		}
	}

	switch status {
	case 400:
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: msg,
		}
	case 403:
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: msg,
		}
	case 410:
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: msg,
		}
	case 429:
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: msg,
		}
	case 500, 502, 503, 504:
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: msg,
		}
	default:
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: msg,
		}
	}
}

func buildPayload(message *domain.Message) ([]byte, error) {
	aps := map[string]any{}
	if message.Title != "" || message.Body != "" {
		alert := map[string]string{}
		if message.Title != "" {
			alert["title"] = message.Title
		}
		if message.Body != "" {
			alert["body"] = message.Body
		}
		aps["alert"] = alert
		sound := message.Sound
		if sound == "" {
			sound = "default"
		}
		aps["sound"] = sound
	}
	if message.Badge != nil {
		aps["badge"] = *message.Badge
	}
	if message.Category != "" {
		aps["category"] = message.Category
	}
	// content-available for data-only background
	if message.Title == "" && message.Body == "" && len(message.Data) > 0 {
		aps["content-available"] = 1
	}

	payload := map[string]any{"aps": aps}
	for k, v := range message.Data {
		if k == "aps" {
			continue
		}
		payload[k] = v
	}
	if message.ID != "" {
		payload["mgl_message_id"] = message.ID
	}
	if message.DeepLink != "" {
		payload["deep_link"] = message.DeepLink
	}
	if message.ImageURL != "" {
		payload["image_url"] = message.ImageURL
	}
	return json.Marshal(payload)
}

func pushType(message *domain.Message) string {
	if message.Title != "" || message.Body != "" {
		return "alert"
	}
	return "background"
}

func apnsPriority(p string) string {
	if p == domain.PriorityHigh {
		return "10"
	}
	// background recommended 5
	return "5"
}

func (p *Provider) bearerToken() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.jwt != "" && time.Now().Before(p.jwtExpiry) {
		return p.jwt, nil
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": p.cfg.TeamID,
		"iat": now.Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	t.Header["kid"] = p.cfg.KeyID
	signed, err := t.SignedString(p.key)
	if err != nil {
		return "", err
	}
	p.jwt = signed
	p.jwtExpiry = now.Add(jwtTTL)
	return signed, nil
}

func loadPrivateKeyPEM(v string) ([]byte, error) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, "BEGIN") {
		return []byte(v), nil
	}
	// Treat as file path
	b, err := os.ReadFile(v)
	if err != nil {
		return nil, fmt.Errorf("apns: read private key: %w", err)
	}
	return b, nil
}

func parseECPrivateKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ec, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not ECDSA private key")
		}
		return ec, nil
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// APNs-ID header prefers UUID format; ULID is fine as custom request id but
// Apple accepts any string — keep as-is for correlation.
func normalizeAPNsID(id string) string {
	return truncate(id, 64)
}
