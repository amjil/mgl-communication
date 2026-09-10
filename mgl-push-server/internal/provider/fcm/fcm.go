package fcm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
	"google.golang.org/api/option"
)

// Provider sends pushes via Firebase Cloud Messaging HTTP v1 (Admin SDK).
type Provider struct {
	client *messaging.Client
}

// NewFromCredentialsFile creates an FCM provider from a service account JSON file.
func NewFromCredentialsFile(ctx context.Context, credentialsFile string) (*Provider, error) {
	if credentialsFile == "" {
		return nil, fmt.Errorf("fcm: credentials file required")
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("fcm: firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("fcm: messaging client: %w", err)
	}
	return &Provider{client: client}, nil
}

// NewWithClient allows injecting a messaging client (tests).
func NewWithClient(client *messaging.Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) Name() string { return domain.ProviderFCM }

func (p *Provider) Close() error { return nil }

func (p *Provider) ValidateToken(_ context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return &provider.ProviderError{
			Provider:     domain.ProviderFCM,
			Code:         domain.ErrCodeInvalidToken,
			Message:      "empty token",
			InvalidToken: true,
		}
	}
	return nil
}

func (p *Provider) Send(ctx context.Context, message *domain.Message, device *domain.Device) (*provider.SendResult, error) {
	if p.client == nil {
		return nil, &provider.ProviderError{
			Provider:  domain.ProviderFCM,
			Code:      domain.ErrCodeProviderNotAvailable,
			Message:   "fcm client not configured",
			Retryable: false,
		}
	}
	if err := p.ValidateToken(ctx, device.Token); err != nil {
		return &provider.SendResult{
			Accepted:     false,
			InvalidToken: true,
			ErrorCode:    domain.ErrCodeInvalidToken,
			ErrorMessage: err.Error(),
		}, nil
	}

	msg := buildMessage(message, device)
	id, err := p.client.Send(ctx, msg)
	if err != nil {
		return mapSendError(err)
	}
	return &provider.SendResult{
		Accepted:          true,
		ProviderMessageID: id,
	}, nil
}

func buildMessage(message *domain.Message, device *domain.Device) *messaging.Message {
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

	out := &messaging.Message{
		Token: device.Token,
		Data:  data,
		Android: &messaging.AndroidConfig{
			Priority: androidPriority(message.Priority),
		},
	}
	if message.TTL > 0 {
		out.Android.TTL = &message.TTL
	}
	if message.CollapseKey != "" {
		out.Android.CollapseKey = message.CollapseKey
	}

	hasNotification := message.Title != "" || message.Body != "" || message.ImageURL != ""
	if hasNotification {
		out.Notification = &messaging.Notification{
			Title:    message.Title,
			Body:     message.Body,
			ImageURL: message.ImageURL,
		}
		androidN := &messaging.AndroidNotification{
			Title: message.Title,
			Body:  message.Body,
			Sound: message.Sound,
			ImageURL: message.ImageURL,
		}
		if message.DeepLink != "" {
			androidN.ClickAction = message.DeepLink
		}
		if message.Category != "" {
			androidN.ChannelID = message.Category
		}
		out.Android.Notification = androidN
	}

	return out
}

func androidPriority(p string) string {
	if p == domain.PriorityHigh {
		return "high"
	}
	return "normal"
}

func mapSendError(err error) (*provider.SendResult, error) {
	msg := err.Error()
	lower := strings.ToLower(msg)

	invalid := messaging.IsUnregistered(err) ||
		strings.Contains(lower, "registration-token-not-registered") ||
		strings.Contains(lower, "invalid-registration-token") ||
		strings.Contains(lower, "unregistered")

	if invalid {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			InvalidToken: true,
			ErrorCode:    domain.ErrCodeInvalidToken,
			ErrorMessage: msg,
		}, nil
	}

	if messaging.IsInvalidArgument(err) {
		// Malformed payload / bad request — do not retry.
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			ErrorCode:    domain.ErrCodeInvalidRequest,
			ErrorMessage: msg,
		}, nil
	}

	if messaging.IsQuotaExceeded(err) || strings.Contains(lower, "quota") || strings.Contains(lower, "429") {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    true,
			ErrorCode:    domain.ErrCodeProviderRateLimit,
			ErrorMessage: msg,
		}, nil
	}

	if messaging.IsUnavailable(err) || messaging.IsInternal(err) ||
		strings.Contains(lower, "503") || strings.Contains(lower, "500") ||
		strings.Contains(lower, "timeout") || strings.Contains(lower, "connection") {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    true,
			ErrorCode:    domain.ErrCodeProviderTempError,
			ErrorMessage: msg,
		}, nil
	}

	if messaging.IsSenderIDMismatch(err) || messaging.IsThirdPartyAuthError(err) ||
		strings.Contains(lower, "authentication") || strings.Contains(lower, "permission") {
		return &provider.SendResult{
			Accepted:     false,
			Retryable:    false,
			ErrorCode:    domain.ErrCodeProviderAuthError,
			ErrorMessage: msg,
		}, nil
	}

	// Unknown: treat as retryable temporary to avoid permanent loss.
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		return nil, pe
	}
	return &provider.SendResult{
		Accepted:     false,
		Retryable:    true,
		ErrorCode:    domain.ErrCodeProviderTempError,
		ErrorMessage: msg,
	}, nil
}
