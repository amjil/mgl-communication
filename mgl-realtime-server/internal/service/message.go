package service

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/idgen"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/metrics"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/repository"
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
	Type            domain.MessageType
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
	IdempotencyKey  string
}

type SendIncomingCallInput struct {
	AppID           string
	UserIDs         []string
	InstallationIDs []string
	Call            domain.IncomingCall
	IdempotencyKey  string
}

type SendCallSignalInput struct {
	AppID           string
	Type            domain.MessageType // call_cancelled | call_ended
	CallID          string
	UserIDs         []string
	InstallationIDs []string
	Reason          string
	IdempotencyKey  string
}

type SendMessageResult struct {
	MessageID string
	Accepted  bool
	Duplicate bool
}

func (s *MessageService) Send(ctx context.Context, in SendMessageInput) (*SendMessageResult, error) {
	if in.Type == "" {
		if in.Title == "" && in.Body == "" && len(in.Data) > 0 {
			in.Type = domain.MessageSilent
		} else {
			in.Type = domain.MessageNotification
		}
	}

	if err := validateSend(in); err != nil {
		return nil, err
	}

	// 1. Resolve devices early; reject invalid targets before persisting.
	devices, err := s.resolveDevices(ctx, in.AppID, in.UserIDs, in.InstallationIDs)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 && in.Provider == "" {
		return nil, domain.InvalidRequest("no active devices found for targets")
	}

	// 2. Initialize the Message entity.
	msg := &domain.Message{
		ID:          idgen.New(),
		Type:        in.Type,
		AppID:       in.AppID,
		Title:       in.Title,
		Body:        in.Body,
		Data:        cloneData(in.Data),
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
	if msg.Priority == "" {
		if msg.Type.IsCallRelated() {
			msg.Priority = domain.PriorityHigh
		} else {
			msg.Priority = domain.PriorityNormal
		}
	}

	// Provide a normalized payload for clients.
	enrichEventData(msg)

	var createdID string

	// 3. Persist safely (with or without idempotency).
	if in.IdempotencyKey != "" {
		actualMsgID, isNew, err := s.messages.CreateWithMessageAndIdempotency(ctx, msg, in.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if !isNew {
			// Concurrent insert or prior record; treat as duplicate.
			return &SendMessageResult{MessageID: actualMsgID, Accepted: true, Duplicate: true}, nil
		}
		createdID = actualMsgID
	} else {
		// Fallback create when no IdempotencyKey is provided.
		created, err := s.messages.Create(ctx, msg)
		if err != nil {
			return nil, err
		}
		createdID = created.ID
	}

	// Align in-memory ID with the persisted ID.
	msg.ID = createdID

	// 4. Continue: direct provider push and/or enqueue.
	if in.Provider != "" && in.Token != "" {
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

	if err := s.enqueueDeliveries(ctx, in.AppID, msg, devices); err != nil {
		return nil, err
	}

	metrics.MessagesTotal.WithLabelValues(in.AppID, string(msg.Type)).Inc()
	return &SendMessageResult{MessageID: createdID, Accepted: true}, nil
}

func (s *MessageService) SendIncomingCall(ctx context.Context, in SendIncomingCallInput) (*SendMessageResult, error) {
	if in.Call.CallID == "" || in.Call.CallerID == "" || in.Call.CalleeID == "" {
		return nil, domain.InvalidRequest("call_id, caller_id, and callee_id are required")
	}
	if in.Call.MediaType == "" {
		in.Call.MediaType = "audio"
	}
	now := time.Now().UTC()
	if in.Call.Timestamp.IsZero() {
		in.Call.Timestamp = now
	}
	if in.Call.ExpiresAt.IsZero() {
		in.Call.ExpiresAt = now.Add(domain.DefaultIncomingCallTTL)
	}

	ttl := int(in.Call.ExpiresAt.Sub(now).Seconds())
	if ttl < 1 {
		return nil, domain.InvalidRequest("call already expired")
	}

	key := in.IdempotencyKey
	if key == "" {
		key = fmt.Sprintf("incoming:%s:%s", in.Call.CallID, joinIDs(in.UserIDs, in.InstallationIDs))
	}

	data := map[string]string{
		"call_id":    in.Call.CallID,
		"caller_id":  in.Call.CallerID,
		"callee_id":  in.Call.CalleeID,
		"media_type": in.Call.MediaType,
		"timestamp":  strconv.FormatInt(in.Call.Timestamp.Unix(), 10),
		"expires_at": strconv.FormatInt(in.Call.ExpiresAt.Unix(), 10),
	}
	if in.Call.CallerName != "" {
		data["caller_display_name"] = in.Call.CallerName
	}

	result, err := s.Send(ctx, SendMessageInput{
		AppID:           in.AppID,
		Type:            domain.MessageIncomingCall,
		UserIDs:         in.UserIDs,
		InstallationIDs: in.InstallationIDs,
		Data:            data,
		Priority:        domain.PriorityHigh,
		TTLSeconds:      &ttl,
		IdempotencyKey:  key,
	})
	if err != nil {
		return nil, err
	}
	metrics.IncomingCallPushTotal.WithLabelValues(in.AppID).Inc()
	return result, nil
}

func (s *MessageService) SendCallSignal(ctx context.Context, in SendCallSignalInput) (*SendMessageResult, error) {
	if in.Type != domain.MessageCallCancelled && in.Type != domain.MessageCallEnded {
		return nil, domain.InvalidRequest("type must be call_cancelled or call_ended")
	}
	if in.CallID == "" {
		return nil, domain.InvalidRequest("call_id is required")
	}
	ttl := 30
	data := map[string]string{
		"call_id": in.CallID,
	}
	if in.Reason != "" {
		data["reason"] = in.Reason
	}
	key := in.IdempotencyKey
	if key == "" {
		key = fmt.Sprintf("%s:%s:%s", in.Type, in.CallID, joinIDs(in.UserIDs, in.InstallationIDs))
	}
	return s.Send(ctx, SendMessageInput{
		AppID:           in.AppID,
		Type:            in.Type,
		UserIDs:         in.UserIDs,
		InstallationIDs: in.InstallationIDs,
		Data:            data,
		Priority:        domain.PriorityHigh,
		TTLSeconds:      &ttl,
		IdempotencyKey:  key,
	})
}

func (s *MessageService) enqueueDeliveries(ctx context.Context, appID string, created *domain.Message, devices []*domain.Device) error {
	devices = filterDevicesForMessage(devices, created.Type)
	var deliveries []*domain.Delivery
	for _, d := range devices {
		provider := selectProviderForMessage(d, created)
		deliveries = append(deliveries, &domain.Delivery{
			MessageID: created.ID,
			DeviceID:  d.ID,
			Provider:  provider,
			Status:    domain.DeliveryStatusPending,
			Attempts:  0,
		})
		metrics.DeliveryTotal.WithLabelValues(appID, provider, d.Platform).Inc()
	}
	if err := s.deliveries.CreateMany(ctx, deliveries); err != nil {
		return err
	}
	return s.messages.MarkQueued(ctx, created.ID)
}

// filterDevicesForMessage avoids duplicate iOS deliveries when both apns and
// apns_voip rows share an installation_id.
func filterDevicesForMessage(devices []*domain.Device, msgType domain.MessageType) []*domain.Device {
	if len(devices) == 0 {
		return devices
	}
	incoming := msgType == domain.MessageIncomingCall ||
		msgType == domain.MessageCallCancelled ||
		msgType == domain.MessageCallEnded

	byInstall := map[string][]*domain.Device{}
	var order []string
	for _, d := range devices {
		key := d.AppID + "|" + d.InstallationID
		if _, ok := byInstall[key]; !ok {
			order = append(order, key)
		}
		byInstall[key] = append(byInstall[key], d)
	}

	var out []*domain.Device
	for _, key := range order {
		group := byInstall[key]
		if len(group) == 1 || group[0].Platform != domain.PlatformIOS {
			out = append(out, group...)
			continue
		}
		var voip, apns, other []*domain.Device
		for _, d := range group {
			switch d.Provider {
			case domain.ProviderAPNsVoIP:
				voip = append(voip, d)
			case domain.ProviderAPNs:
				apns = append(apns, d)
			default:
				other = append(other, d)
			}
		}
		if incoming {
			if len(voip) > 0 {
				out = append(out, voip...)
			} else {
				out = append(out, apns...)
				out = append(out, other...)
			}
		} else {
			if len(apns) > 0 {
				out = append(out, apns...)
			} else if len(other) > 0 {
				out = append(out, other...)
			} else {
				out = append(out, voip...)
			}
		}
	}
	return out
}

func selectProviderForMessage(d *domain.Device, msg *domain.Message) string {
	// iOS incoming call prefers VoIP transport when device registered as apns_voip or apns.
	if d.Platform == domain.PlatformIOS && msg.Type == domain.MessageIncomingCall {
		if d.Provider == domain.ProviderAPNsVoIP {
			return domain.ProviderAPNsVoIP
		}
		// Fall back to normal APNs with high priority (PushKit token may be separate later).
		return domain.ProviderAPNs
	}
	return d.Provider
}

func enrichEventData(msg *domain.Message) {
	if msg.Data == nil {
		msg.Data = map[string]string{}
	}
	msg.Data["mgl_message_id"] = msg.ID
	msg.Data["mgl_event_id"] = msg.ID
	msg.Data["mgl_event_type"] = string(msg.Type)
	msg.Data["mgl_timestamp"] = strconv.FormatInt(time.Now().UTC().Unix(), 10)
}

func (s *MessageService) resolveDevices(ctx context.Context, appID string, userIDs, installationIDs []string) ([]*domain.Device, error) {
	seen := map[string]bool{}
	var out []*domain.Device

	if len(userIDs) > 0 {
		ds, err := s.devices.ListActiveByUserIDs(ctx, appID, userIDs)
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
	if len(installationIDs) > 0 {
		ds, err := s.devices.ListActiveByInstallationIDs(ctx, appID, installationIDs)
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
	switch in.Type {
	case domain.MessageNotification, domain.MessageSilent, domain.MessageBackground,
		domain.MessageIncomingCall, domain.MessageCallCancelled, domain.MessageCallEnded:
	default:
		return domain.InvalidRequest("invalid message type")
	}
	if in.Type == domain.MessageNotification && in.Title == "" && in.Body == "" && len(in.Data) == 0 {
		return domain.InvalidRequest("notification or data required")
	}
	if in.Type.IsDataOnly() && len(in.Data) == 0 {
		return domain.InvalidRequest("data required for this message type")
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
	switch p {
	case domain.ProviderAPNs, domain.ProviderAPNsVoIP:
		return domain.PlatformIOS
	default:
		return domain.PlatformAndroid
	}
}

func cloneData(in map[string]string) map[string]string {
	if in == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func joinIDs(userIDs, installationIDs []string) string {
	if len(userIDs) > 0 {
		return userIDs[0]
	}
	if len(installationIDs) > 0 {
		return installationIDs[0]
	}
	return "all"
}
