package oppo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
)

const (
	defaultHost     = "https://api.push.oppomobile.com"
	authPath        = "/server/v1/auth"
	unicastPath     = "/server/v1/message/notification/unicast"
	defaultChannel  = "mgl_default"
)

type Config struct {
	AppKey       string
	MasterSecret string
	Host         string // override for tests
}

type Provider struct {
	cfg        Config
	httpClient *http.Client

	mu         sync.Mutex
	authToken  string
	authExpiry time.Time
}

func New(cfg Config) (*Provider, error) {
	if cfg.AppKey == "" || cfg.MasterSecret == "" {
		return nil, fmt.Errorf("oppo: app_key and master_secret required")
	}
	if cfg.Host == "" {
		cfg.Host = defaultHost
	}
	return &Provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *Provider) Name() string { return domain.ProviderOppo }

func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return &provider.ProviderError{
			Provider:     domain.ProviderOppo,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "empty oppo registration id",
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

	token, err := p.getAuthToken(ctx)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: err.Error(),
		}, nil
	}

	msgJSON, err := buildMessageJSON(message, device.Token)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: err.Error(),
		}, nil
	}

	form := url.Values{}
	form.Set("auth_token", token)
	form.Set("message", string(msgJSON))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Host+unicastPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: err.Error(),
		}, nil
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		p.invalidateAuth()
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: string(body),
		}, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: string(body),
		}, nil
	}
	if res.StatusCode >= 500 {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: string(body),
		}, nil
	}

	return mapResponse(body), nil
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type sendData struct {
	MessageID string `json:"messageId"`
	TaskID    string `json:"task_id"`
}

func mapResponse(body []byte) *provider.SendResult {
	var ar apiResponse
	_ = json.Unmarshal(body, &ar)
	msg := ar.Message
	if msg == "" {
		msg = string(body)
	}

	var sd sendData
	if len(ar.Data) > 0 {
		_ = json.Unmarshal(ar.Data, &sd)
	}
	msgID := sd.MessageID
	if msgID == "" {
		msgID = sd.TaskID
	}

	switch ar.Code {
	case 0:
		return &provider.SendResult{Accepted: true, ProviderMessageID: msgID}
	case 11, 33, 10000: // invalid registration / invalid target
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: msg,
		}
	case 12, 13: // auth issues
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: msg,
		}
	case 31, 41: // rate / throttle style
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: msg,
		}
	}

	lower := strings.ToLower(msg)
	if strings.Contains(lower, "registration") || strings.Contains(lower, "regid") || strings.Contains(lower, "invalid target") {
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: msg,
		}
	}

	return &provider.SendResult{
		Accepted: false, Retryable: true,
		ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: msg,
	}
}

func buildMessageJSON(message *domain.Message, regID string) ([]byte, error) {
	title := message.Title
	body := message.Body
	if title == "" && body == "" {
		// OPPO unicast API requires a notification shell; keep payload in action_parameters.
		switch message.Type {
		case domain.MessageIncomingCall:
			title = "Incoming call"
			if name, ok := message.Data["caller_display_name"]; ok && name != "" {
				body = name
			} else if id, ok := message.Data["caller_id"]; ok {
				body = id
			} else {
				body = " "
			}
		case domain.MessageCallCancelled:
			title = "Call cancelled"
			body = message.Data["call_id"]
			if body == "" {
				body = " "
			}
		case domain.MessageCallEnded:
			title = "Call ended"
			body = message.Data["call_id"]
			if body == "" {
				body = " "
			}
		default:
			title = "Notification"
			if t, ok := message.Data["mgl_event_type"]; ok {
				body = t
			} else if t, ok := message.Data["type"]; ok {
				body = t
			} else {
				body = " "
			}
		}
	}

	channel := message.Category
	if channel == "" {
		channel = defaultChannel
	}

	notification := map[string]any{
		"style":        1,
		"title":        title,
		"content":      body,
		"channel_id":   channel,
		"off_line":     true,
		"push_time_type": 0,
	}
	if message.TTL > 0 {
		notification["off_line_ttl"] = int(message.TTL.Seconds())
	} else {
		notification["off_line_ttl"] = 86400
	}
	if message.CollapseKey != "" {
		notification["notify_id"] = hashNotifyID(message.CollapseKey)
	}

	// click action: open app by default; deep link via intent
	clickAction := map[string]any{
		"action": 0, // 0 launch app
	}
	if message.DeepLink != "" {
		clickAction = map[string]any{
			"action":      1, // intent
			"action_type": 1,
			"intent":      message.DeepLink,
			"url":         message.DeepLink,
		}
		notification["click_action_type"] = 1
		notification["click_action_activity"] = message.DeepLink
		notification["click_action_url"] = message.DeepLink
	}
	_ = clickAction

	data := map[string]string{}
	for k, v := range message.Data {
		data[k] = v
	}
	if message.ID != "" {
		data["mgl_message_id"] = message.ID
		data["mgl_event_id"] = message.ID
	}
	if message.Type != "" {
		data["mgl_event_type"] = string(message.Type)
	}
	if message.DeepLink != "" {
		data["deep_link"] = message.DeepLink
	}
	if len(data) > 0 {
		b, _ := json.Marshal(data)
		notification["action_parameters"] = string(b)
	}

	msg := map[string]any{
		"target_type":            2, // registration_id
		"target_value":           regID,
		"verify_registration_id": true,
		"notification":           notification,
	}
	return json.Marshal(msg)
}

func (p *Provider) getAuthToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authToken != "" && time.Now().Before(p.authExpiry) {
		return p.authToken, nil
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sign := sha256Hex(p.cfg.AppKey + ts + p.cfg.MasterSecret)

	form := url.Values{}
	form.Set("app_key", p.cfg.AppKey)
	form.Set("timestamp", ts)
	form.Set("sign", sign)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Host+authPath, strings.NewReader(form.Encode()))
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
		return "", fmt.Errorf("oppo auth status=%d body=%s", res.StatusCode, string(body))
	}

	var ar struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AuthToken  string `json:"auth_token"`
			CreateTime int64  `json:"create_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ar); err != nil {
		return "", err
	}
	if ar.Code != 0 || ar.Data.AuthToken == "" {
		return "", fmt.Errorf("oppo auth code=%d message=%s", ar.Code, ar.Message)
	}
	p.authToken = ar.Data.AuthToken
	// Token valid ~24h; refresh early
	p.authExpiry = time.Now().Add(20 * time.Hour)
	return p.authToken, nil
}

func (p *Provider) invalidateAuth() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authToken = ""
	p.authExpiry = time.Time{}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hashNotifyID(s string) int {
	var h int32
	for i := 0; i < len(s); i++ {
		h = 31*h + int32(s[i])
	}
	if h < 0 {
		h = -h
	}
	return int(h % 100000000)
}
