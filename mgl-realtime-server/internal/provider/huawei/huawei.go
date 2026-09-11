package huawei

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider"
)

const (
	defaultTokenURL = "https://oauth-login.cloud.huawei.com/oauth2/v3/token"
	defaultPushURL  = "https://push-api.cloud.huawei.com/v1/%s/messages:send"
)

type Config struct {
	AppID     string
	AppSecret string
	// Optional overrides for tests.
	TokenURL string
	PushURL  string // must contain %s for appId if using default pattern; or full URL for tests
}

type Provider struct {
	cfg        Config
	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

func New(cfg Config) (*Provider, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("huawei: app_id and app_secret required")
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	if cfg.PushURL == "" {
		cfg.PushURL = fmt.Sprintf(defaultPushURL, cfg.AppID)
	}
	return &Provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *Provider) Name() string { return domain.ProviderHuawei }

func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return &provider.ProviderError{
			Provider:     domain.ProviderHuawei,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "empty huawei token",
			InvalidToken: true,
		}
	}
	return nil
}

func (p *Provider) Send(ctx context.Context, message *domain.Message, device *domain.Device) (*provider.SendResult, error) {
	if err := p.ValidateToken(ctx, device.Token); err != nil {
		return &provider.SendResult{
			Accepted: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: err.Error(),
		}, nil
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: err.Error(),
		}, nil
	}

	body, err := buildRequestBody(message, device.Token)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: err.Error(),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.PushURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := p.httpClient.Do(req)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: err.Error(),
		}, nil
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		p.invalidateToken()
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: string(respBody),
		}, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: string(respBody),
		}, nil
	}
	if res.StatusCode >= 500 {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: string(respBody),
		}, nil
	}

	return mapPushResponse(res.StatusCode, respBody), nil
}

type sendResponse struct {
	Code      string `json:"code"`
	Msg       string `json:"msg"`
	RequestID string `json:"requestId"`
}

func mapPushResponse(status int, body []byte) *provider.SendResult {
	var sr sendResponse
	_ = json.Unmarshal(body, &sr)
	code := sr.Code
	msg := sr.Msg
	if msg == "" {
		msg = string(body)
	}

	switch code {
	case "80000000", "80000001": // success / some success
		return &provider.SendResult{
			Accepted:          true,
			ProviderMessageID: sr.RequestID,
		}
	case "80300007", "80100003", "80200001", "80200003":
		// all tokens invalid / illegal token
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: msg,
		}
	case "80100000":
		// partial success — treat as accepted for at-least-once single-token sends
		return &provider.SendResult{
			Accepted:          true,
			ProviderMessageID: sr.RequestID,
			ErrorMessage:      msg,
		}
	case "80300002", "80300008":
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: msg,
		}
	case "80300010":
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: msg,
		}
	}

	if status >= 200 && status < 300 && code == "" {
		return &provider.SendResult{Accepted: true, ProviderMessageID: sr.RequestID}
	}
	retryable := status >= 500 || code == ""
	return &provider.SendResult{
		Accepted: false, Retryable: retryable,
		ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: msg,
	}
}

func buildRequestBody(message *domain.Message, deviceToken string) ([]byte, error) {
	dataMap := map[string]string{}
	for k, v := range message.Data {
		dataMap[k] = v
	}
	if message.ID != "" {
		dataMap["mgl_message_id"] = message.ID
		dataMap["mgl_event_id"] = message.ID
	}
	if message.Type != "" {
		dataMap["mgl_event_type"] = string(message.Type)
	}
	if message.DeepLink != "" {
		dataMap["deep_link"] = message.DeepLink
	}
	dataJSON, err := json.Marshal(dataMap)
	if err != nil {
		return nil, err
	}

	msg := map[string]any{
		"token": []string{deviceToken},
		"data":  string(dataJSON),
	}

	android := map[string]any{}
	if message.TTL > 0 {
		android["ttl"] = fmt.Sprintf("%ds", int(message.TTL.Seconds()))
	}
	// Incoming call / high priority must set urgency even for data-only (Phase 2).
	if message.Priority == domain.PriorityHigh || message.Type.IsCallRelated() {
		android["urgency"] = "HIGH"
	}
	if message.CollapseKey != "" && !message.Type.IsCallRelated() {
		android["bi_tag"] = message.CollapseKey
	}

	hasNotification := !message.Type.IsDataOnly() && (message.Title != "" || message.Body != "")
	if hasNotification {
		msg["notification"] = map[string]any{
			"title": message.Title,
			"body":  message.Body,
			"image": message.ImageURL,
		}
		androidN := map[string]any{
			"title": message.Title,
			"body":  message.Body,
			"click_action": map[string]any{
				"type": 3, // open app
			},
		}
		if message.ImageURL != "" {
			androidN["image"] = message.ImageURL
		}
		if message.Sound != "" {
			androidN["sound"] = message.Sound
		}
		if message.Category != "" {
			androidN["channel_id"] = message.Category
			androidN["importance"] = "NORMAL"
		}
		if message.DeepLink != "" {
			androidN["click_action"] = map[string]any{
				"type":   1,
				"intent": message.DeepLink,
			}
		}
		android["notification"] = androidN
	}

	if len(android) > 0 {
		msg["android"] = android
	}

	payload := map[string]any{
		"validate_only": false,
		"message":       msg,
	}
	return json.Marshal(payload)
}

func (p *Provider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.accessToken != "" && time.Now().Before(p.expiry) {
		return p.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.cfg.AppID)
	form.Set("client_secret", p.cfg.AppSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("huawei oauth status=%d body=%s", res.StatusCode, string(body))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("huawei oauth: empty access_token")
	}
	p.accessToken = tr.AccessToken
	// Refresh a bit early
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 55 * time.Minute
	}
	p.expiry = time.Now().Add(ttl - 60*time.Second)
	return p.accessToken, nil
}

func (p *Provider) invalidateToken() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accessToken = ""
	p.expiry = time.Time{}
}
