package service

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/idgen"
	"github.com/amjil/mgl-push/mgl-push-server/internal/metrics"
	"github.com/amjil/mgl-push/mgl-push-server/internal/repository"
)

const maxDataEntries = 32
const maxDataValueLen = 1024

type MessageService struct {
	messages   *repository.MessageRepository
	deliveries *repository.DeliveryRepository
	devices    *repository.DeviceRepository
}

func NewMessageService(
	messages *repository.MessageRepository,
	deliveries *repository.DeliveryRepository,
	devices *repository.DeviceRepository,
) *MessageService {
	return &MessageService{
		messages:   messages,
		deliveries: deliveries,
		devices:    devices,
	}
}

type SendMessageInput struct {
	AppID           string
	UserIDs         []string
	InstallationIDs []string
	Provider        string
	Token           string
	Title           string
	Body            string
	Data            map[string]string
	ImageURL        string
	Priority        string
	TTLSeconds      *int
	CollapseKey     string
	Sound           string
	Badge           *int
	DeepLink        string
	Category        string
}

type SendMessageResult struct {
	MessageID string
	Accepted  bool
}

func (s *MessageService) Send(ctx context.Context, in SendMessageInput) (*SendMessageResult, error) {
	if err := validateSend(in); err != nil {
		return nil, err
	}

	devices, err := s.resolveDevices(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 && in.Provider == "" {
		return nil, domain.InvalidRequest("no active devices found for targets")
	}

	msg := &domain.Message{
		ID:          idgen.New(),
		AppID:       in.AppID,
		Title:       in.Title,
		Body:        in.Body,
		Data:        in.Data,
		ImageURL:    in.ImageURL,
		Priority:    in.Priority,
		CollapseKey: in.CollapseKey,
		Sound:       in.Sound,
		Badge:       in.Badge,
		DeepLink:    in.DeepLink,
		Category:    in.Category,
		Status:      domain.MessageStatusCreated,
	}
	if in.TTLSeconds != nil {
		msg.TTL = time.Duration(*in.TTLSeconds) * time.Second
	}
	if msg.Data == nil {
		msg.Data = map[string]string{}
	}
	// Always include mgl_message_id for client dedup
	msg.Data["mgl_message_id"] = msg.ID

	created, err := s.messages.Create(ctx, msg)
	if err != nil {
		return nil, err
	}

	var deliveries []*domain.Delivery
	if in.Provider != "" && in.Token != "" {
		// Direct token send — create ephemeral device-backed delivery via synthetic device row is not required;
		// store a delivery with a placeholder device created on the fly is complex. Instead upsert a transient
		// approach: create delivery pointing to a device we temporarily register is out of scope.
		// Phase 1: resolve by creating an ephemeral in-memory path through a temporary device upsert.
		tmp, err := s.devices.Upsert(ctx, &domain.Device{
			InstallationID: "direct-" + idgen.New(),
			Platform:       platformForProvider(in.Provider),
			Provider:       in.Provider,
			Token:          in.Token,
			AppID:          in.AppID,
			Status:         domain.DeviceStatusActive,
		})
		if err != nil {
			return nil, err
		}
		devices = []*domain.Device{tmp}
	}

	for _, d := range devices {
		deliveries = append(deliveries, &domain.Delivery{
			MessageID: created.ID,
			DeviceID:  d.ID,
			Provider:  d.Provider,
			Status:    domain.DeliveryStatusPending,
			Attempts:  0,
		})
		metrics.DeliveryTotal.WithLabelValues(in.AppID, d.Provider, d.Platform).Inc()
	}

	if err := s.deliveries.CreateMany(ctx, deliveries); err != nil {
		return nil, err
	}
	if err := s.messages.MarkQueued(ctx, created.ID); err != nil {
		return nil, err
	}

	metrics.MessagesTotal.WithLabelValues(in.AppID, "").Inc()

	return &SendMessageResult{MessageID: created.ID, Accepted: true}, nil
}

func (s *MessageService) resolveDevices(ctx context.Context, in SendMessageInput) ([]*domain.Device, error) {
	seen := map[string]bool{}
	var out []*domain.Device

	if len(in.UserIDs) > 0 {
		ds, err := s.devices.ListActiveByUserIDs(ctx, in.AppID, in.UserIDs)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			if !seen[d.ID] {
				seen[d.ID] = true
				out = append(out, d)
			}
		}
	}
	if len(in.InstallationIDs) > 0 {
		ds, err := s.devices.ListActiveByInstallationIDs(ctx, in.AppID, in.InstallationIDs)
		if err != nil {
			return nil, err
		}
		for _, d := range ds {
			if !seen[d.ID] {
				seen[d.ID] = true
				out = append(out, d)
			}
		}
	}
	return out, nil
}

func validateSend(in SendMessageInput) error {
	if in.AppID == "" {
		return domain.InvalidRequest("app_id is required")
	}
	hasTarget := len(in.UserIDs) > 0 || len(in.InstallationIDs) > 0 || (in.Provider != "" && in.Token != "")
	if !hasTarget {
		return domain.InvalidRequest("user_ids, installation_ids, or provider+token required")
	}
	if in.Title == "" && in.Body == "" && len(in.Data) == 0 {
		return domain.InvalidRequest("notification or data required")
	}
	if in.Priority != "" && in.Priority != domain.PriorityNormal && in.Priority != domain.PriorityHigh {
		return domain.InvalidRequest("priority must be normal or high")
	}
	if in.Provider != "" && !allowedProviders[in.Provider] {
		return domain.InvalidRequest("invalid provider")
	}
	if len(in.Data) > maxDataEntries {
		return domain.InvalidRequest("data has too many keys")
	}
	for k, v := range in.Data {
		if k == "" || utf8.RuneCountInString(v) > maxDataValueLen {
			return domain.InvalidRequest("invalid data payload")
		}
	}
	return nil
}

func platformForProvider(p string) string {
	if p == domain.ProviderAPNs {
		return domain.PlatformIOS
	}
	return domain.PlatformAndroid
}
