package xiaomi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
)

const defaultSendURL = "https://api.xmpush.xiaomi.com/v3/message/regid"

// Config for MiPush HTTP API (registration_id path — Phase 5 priority).
type Config struct {
	AppSecret string
	// Optional package name fallback when device.AppID is empty.
	PackageName string
	SendURL     string // override for tests
}

type Provider struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config) (*Provider, error) {
	if cfg.AppSecret == "" {
		return nil, fmt.Errorf("xiaomi: app_secret required")
	}
	if cfg.SendURL == "" {
		cfg.SendURL = defaultSendURL
	}
	return &Provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (p *Provider) Name() string { return domain.ProviderXiaomi }

func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return &provider.ProviderError{
			Provider:     domain.ProviderXiaomi,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "empty xiaomi registration id",
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

	form, err := buildForm(message, device, p.cfg.PackageName)
	if err != nil {
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: err.Error(),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.SendURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "key="+p.cfg.AppSecret)

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
		return &provider.SendResult{
			Accepted: false, Retryable: false,
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

	return mapResponse(res.StatusCode, body), nil
}

type apiResponse struct {
	Result      string          `json:"result"`
	Reason      string          `json:"reason"`
	Code        int             `json:"code"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	Info        string          `json:"info"`
	TraceID     string          `json:"trace_id"`
}

type apiData struct {
	ID string `json:"id"`
}

func mapResponse(status int, body []byte) *provider.SendResult {
	var ar apiResponse
	_ = json.Unmarshal(body, &ar)

	msgID := ""
	var d apiData
	if len(ar.Data) > 0 {
		_ = json.Unmarshal(ar.Data, &d)
		msgID = d.ID
	}
	if msgID == "" && ar.TraceID != "" {
		msgID = ar.TraceID
	}

	result := strings.ToLower(ar.Result)
	reason := strings.ToLower(ar.Reason + " " + ar.Description + " " + ar.Info)

	if result == "ok" || ar.Code == 0 {
		return &provider.SendResult{
			Accepted:          true,
			ProviderMessageID: msgID,
		}
	}

	invalid := strings.Contains(reason, "invalid registration") ||
		(strings.Contains(reason, "regid") && strings.Contains(reason, "invalid")) ||
		strings.Contains(reason, "not registered") ||
		ar.Code == 20301 || ar.Code == 20302 || ar.Code == 10001

	if invalid {
		return &provider.SendResult{
			Accepted: false, Retryable: false, InvalidToken: true,
			ErrorCode: domain.ErrCodeInvalidToken, ErrorMessage: string(body),
		}
	}

	if ar.Code == 10002 || strings.Contains(reason, "quota") || strings.Contains(reason, "rate") {
		return &provider.SendResult{
			Accepted: false, Retryable: true,
			ErrorCode: domain.ErrCodeProviderRateLimit, ErrorMessage: string(body),
		}
	}

	if ar.Code == 10008 || strings.Contains(reason, "auth") || strings.Contains(reason, "secret") {
		return &provider.SendResult{
			Accepted: false, Retryable: false,
			ErrorCode: domain.ErrCodeProviderAuthError, ErrorMessage: string(body),
		}
	}

	retryable := status >= 500 || result == "" || result == "error"
	return &provider.SendResult{
		Accepted: false, Retryable: retryable,
		ErrorCode: domain.ErrCodeProviderTempError, ErrorMessage: string(body),
	}
}

func buildForm(message *domain.Message, device *domain.Device, fallbackPkg string) (url.Values, error) {
	form := url.Values{}
	form.Set("registration_id", device.Token)

	pkg := device.AppID
	if pkg == "" {
		pkg = fallbackPkg
	}
	if pkg == "" {
		return nil, fmt.Errorf("xiaomi: restricted_package_name (app_id) required")
	}
	form.Set("restricted_package_name", pkg)

	data := map[string]string{}
	for k, v := range message.Data {
		data[k] = v
	}
	if message.ID != "" {
		data["mgl_message_id"] = message.ID
	}
	if message.DeepLink != "" {
		data["deep_link"] = message.DeepLink
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	form.Set("payload", string(payload))

	hasNotification := message.Title != "" || message.Body != ""
	if hasNotification {
		form.Set("title", message.Title)
		form.Set("description", message.Body)
		form.Set("pass_through", "0") // notification
		form.Set("notify_type", "-1") // default sound/vibrate
		if message.DeepLink != "" {
			form.Set("extra.intent_uri", message.DeepLink)
		}
		if message.Sound != "" {
			form.Set("extra.sound_uri", message.Sound)
		}
		if message.Category != "" {
			form.Set("extra.channel_id", message.Category)
		}
	} else {
		form.Set("pass_through", "1") // data-only
	}

	if message.TTL > 0 {
		// Xiaomi expects milliseconds
		form.Set("time_to_live", strconv.FormatInt(message.TTL.Milliseconds(), 10))
	}
	if message.CollapseKey != "" {
		form.Set("notify_id", hashNotifyID(message.CollapseKey))
	}
	if message.Priority == domain.PriorityHigh {
		form.Set("extra.cb", "1")
	}

	return form, nil
}

func hashNotifyID(s string) string {
	// Xiaomi notify_id is int32; derive stable small id from collapse key.
	var h int32
	for i := 0; i < len(s); i++ {
		h = 31*h + int32(s[i])
	}
	if h < 0 {
		h = -h
	}
	return strconv.FormatInt(int64(h%100000000), 10)
}
