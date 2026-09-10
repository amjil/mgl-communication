package vivo

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
	"github.com/amjil/mgl-push/mgl-push-server/internal/idgen"
)

const (
	defaultHost = "https://api-push.vivo.com.cn"
	authPath    = "/message/auth"
	sendPath    = "/message/send"
)

type Config struct {
	AppID     string
	AppKey    string
	AppSecret string
	Host      string // override for tests
	PushMode  int    // 0 official, 1 test
}

type Provider struct {
	cfg        Config
	httpClient *http.Client
	appIDInt   int

	mu         sync.Mutex
	authToken  string
	authExpiry time.Time
}

func New(cfg Config) (*Provider, error) {
	if cfg.AppID == "" || cfg.AppKey == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("vivo: app_id, app_key and app_secret required")
	}
	appIDInt, err := strconv.Atoi(strings.TrimSpace(cfg.AppID))
	if err != nil {
		return nil, fmt.Errorf("vivo: app_id must be numeric: %w", err)
	}
	if cfg.Host == "" {
		cfg.Host = defaultHost
	}
	return &Provider{
		cfg:      cfg,
		appIDInt: appIDInt,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *Provider) Name() string { return domain.ProviderVivo }

func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return &provider.ProviderError{
			Provider:     domain.ProviderVivo,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "empty vivo regId",
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

	body, err := buildSendBody(message, device.Token, p.cfg.PushMode)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: err.Error(),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Host+sendPath, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("authToken", token)

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
		p.invalidateAuth()
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: string(respBody),
		}, nil
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500 {
		code := domain.ErrCodeProviderTempError
		if res.StatusCode == http.StatusTooManyRequests {
			code = domain.ErrCodeProviderRateLimit
		}
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: code, ErrorMessage: string(respBody),
		}, nil
	}

	return mapResponse(respBody), nil
}

type apiResponse struct {
	Result  int    `json:"result"`
	Desc    string `json:"desc"`
	TaskID  string `json:"taskId"`
	Message string `json:"message"`
}

func mapResponse(body []byte) *provider.SendResult {
	var ar apiResponse
	_ = json.Unmarshal(body, &ar)
	msg := ar.Desc
	if msg == "" {
		msg = ar.Message
	}
	if msg == "" {
		msg = string(body)
	}

	switch ar.Result {
	case 0:
		return &provider.SendResult{Accepted: true, ProviderMessageID: ar.TaskID}
	case 10302, 10307: // invalid regId / alias
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: msg,
		}
	case 10000: // auth failed
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: msg,
		}
	case 10070, 10093: // daily limit / rate limit
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: msg,
		}
	}

	lower := strings.ToLower(msg)
	// Vendor error text may include Chinese "不合法" (invalid).
	if strings.Contains(lower, "regid") || strings.Contains(lower, "不合法") {
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

func buildSendBody(message *domain.Message, regID string, pushMode int) ([]byte, error) {
	title := message.Title
	content := message.Body
	if title == "" && content == "" {
		title = "Notification"
		if t, ok := message.Data["type"]; ok {
			content = t
		} else {
			content = " "
		}
	}

	custom := map[string]string{}
	for k, v := range message.Data {
		custom[k] = v
	}
	if message.ID != "" {
		custom["mgl_message_id"] = message.ID
	}
	if message.DeepLink != "" {
		custom["deep_link"] = message.DeepLink
	}

	skipType := 1 // open app home
	skipContent := ""
	if message.DeepLink != "" {
		skipType = 3 // custom — client handles in click callback
		skipContent = message.DeepLink
	}

	ttl := 86400
	if message.TTL > 0 {
		ttl = int(message.TTL.Seconds())
	}

	reqID := message.ID
	if reqID == "" {
		reqID = idgen.New()
	}
	if len(reqID) > 64 {
		reqID = reqID[:64]
	}

	payload := map[string]any{
		"regId":           regID,
		"notifyType":      4, // sound + vibrate
		"title":           title,
		"content":         content,
		"timeToLive":      ttl,
		"skipType":        skipType,
		"requestId":       reqID,
		"pushMode":        pushMode,
		"clientCustomMap": custom,
	}
	if skipContent != "" {
		payload["skipContent"] = skipContent
	}
	if message.Category != "" {
		payload["category"] = message.Category
	}
	return json.Marshal(payload)
}

func (p *Provider) getAuthToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.authToken != "" && time.Now().Before(p.authExpiry) {
		return p.authToken, nil
	}

	ts := time.Now().UnixMilli()
	signSrc := fmt.Sprintf("%d%s%d%s", p.appIDInt, p.cfg.AppKey, ts, p.cfg.AppSecret)
	sign := md5Hex(signSrc)

	body, _ := json.Marshal(map[string]any{
		"appId":     p.appIDInt,
		"appKey":    p.cfg.AppKey,
		"timestamp": ts,
		"sign":      sign,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Host+authPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vivo auth status=%d body=%s", res.StatusCode, string(respBody))
	}

	var ar struct {
		Result    int    `json:"result"`
		Desc      string `json:"desc"`
		AuthToken string `json:"authToken"`
	}
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", err
	}
	if ar.Result != 0 || ar.AuthToken == "" {
		return "", fmt.Errorf("vivo auth result=%d desc=%s", ar.Result, ar.Desc)
	}
	p.authToken = ar.AuthToken
	p.authExpiry = time.Now().Add(2 * time.Hour) // refresh early within 24h validity
	return p.authToken, nil
}

func (p *Provider) invalidateAuth() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.authToken = ""
	p.authExpiry = time.Time{}
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:])
}
