package queue

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
	"github.com/amjil/mgl-push/mgl-push-server/internal/metrics"
	"github.com/amjil/mgl-push/mgl-push-server/internal/provider"
	"github.com/amjil/mgl-push/mgl-push-server/internal/repository"
	"github.com/amjil/mgl-push/mgl-push-server/internal/retry"
)

type Worker struct {
	messages   *repository.MessageRepository
	deliveries *repository.DeliveryRepository
	devices    *repository.DeviceRepository
	registry   *provider.Registry
	interval   time.Duration
	batchSize  int
	maxAttempt int
	logger     *slog.Logger

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewWorker(
	messages *repository.MessageRepository,
	deliveries *repository.DeliveryRepository,
	devices *repository.DeviceRepository,
	registry *provider.Registry,
	interval time.Duration,
	batchSize, maxAttempt int,
	logger *slog.Logger,
) *Worker {
	return &Worker{
		messages:   messages,
		deliveries: deliveries,
		devices:    devices,
		registry:   registry,
		interval:   interval,
		batchSize:  batchSize,
		maxAttempt: maxAttempt,
		logger:     logger,
	}
}

func (w *Worker) Start(parent context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.running = true
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.loop(ctx)
	}()
}

func (w *Worker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.cancel()
	w.running = false
	w.mu.Unlock()
	w.wg.Wait()
}

func (w *Worker) Alive() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.Error("worker tick failed", "error", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	deliveries, err := w.deliveries.ClaimReady(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, d := range deliveries {
		w.processDelivery(ctx, d)
	}
	return nil
}

func (w *Worker) processDelivery(ctx context.Context, d *domain.Delivery) {
	device, err := w.devices.FindByID(ctx, d.DeviceID)
	if err != nil {
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeDeviceNotFound, err.Error())
		return
	}
	msg, err := w.messages.FindByID(ctx, device.AppID, d.MessageID)
	if err != nil {
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeMessageNotFound, err.Error())
		return
	}

	p, err := w.registry.Get(d.Provider)
	if err != nil {
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeProviderNotAvailable, err.Error())
		w.maybeCompleteMessage(ctx, msg.ID)
		return
	}

	start := time.Now()
	result, sendErr := p.Send(ctx, msg, device)
	metrics.ProviderLatency.WithLabelValues(d.Provider).Observe(time.Since(start).Seconds())

	w.logger.Info("delivery attempt",
		"message_id", msg.ID,
		"delivery_id", d.ID,
		"device_id", device.ID,
		"installation_id", device.InstallationID,
		"provider", d.Provider,
		"attempt", d.Attempts,
	)

	if sendErr != nil {
		w.handleError(ctx, d, device, msg, sendErr)
		return
	}
	if result == nil {
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeInternalError, "nil result")
		w.maybeCompleteMessage(ctx, msg.ID)
		return
	}
	if result.InvalidToken {
		_ = w.devices.MarkInvalid(ctx, device.ID)
		metrics.InvalidTokensTotal.WithLabelValues(d.Provider, device.AppID).Inc()
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeInvalidToken, result.ErrorMessage)
		metrics.DeliveryFailedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
		w.maybeCompleteMessage(ctx, msg.ID)
		return
	}
	if !result.Accepted {
		if retry.ShouldRetry(result.Retryable, d.Attempts, w.maxAttempt) {
			next := time.Now().UTC().Add(retry.DelayForAttempt(d.Attempts + 1))
			_ = w.deliveries.MarkRetrying(ctx, d.ID, result.ErrorCode, result.ErrorMessage, next)
			metrics.MessagesRetriedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
			return
		}
		_ = w.deliveries.MarkFailed(ctx, d.ID, result.ErrorCode, result.ErrorMessage)
		metrics.DeliveryFailedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
		metrics.MessagesFailedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
		w.maybeCompleteMessage(ctx, msg.ID)
		return
	}

	_ = w.deliveries.MarkAccepted(ctx, d.ID, result.ProviderMessageID)
	metrics.MessagesAcceptedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
	w.logger.Info("delivery accepted",
		"message_id", msg.ID,
		"delivery_id", d.ID,
		"provider", d.Provider,
		"attempt", d.Attempts,
		"status", "accepted",
	)
	w.maybeCompleteMessage(ctx, msg.ID)
}

func (w *Worker) handleError(ctx context.Context, d *domain.Delivery, device *domain.Device, msg *domain.Message, sendErr error) {
	code := domain.ErrCodeProviderTempError
	retryable := true
	invalid := false
	if pe, ok := sendErr.(*provider.ProviderError); ok {
		code = pe.Code
		retryable = pe.Retryable
		invalid = pe.InvalidToken
	}
	metrics.ProviderErrors.WithLabelValues(d.Provider, code).Inc()

	if invalid {
		_ = w.devices.MarkInvalid(ctx, device.ID)
		metrics.InvalidTokensTotal.WithLabelValues(d.Provider, device.AppID).Inc()
		_ = w.deliveries.MarkFailed(ctx, d.ID, domain.ErrCodeInvalidToken, sendErr.Error())
		w.maybeCompleteMessage(ctx, msg.ID)
		return
	}
	if retry.ShouldRetry(retryable, d.Attempts, w.maxAttempt) {
		next := time.Now().UTC().Add(retry.DelayForAttempt(d.Attempts + 1))
		_ = w.deliveries.MarkRetrying(ctx, d.ID, code, sendErr.Error(), next)
		metrics.MessagesRetriedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
		return
	}
	_ = w.deliveries.MarkFailed(ctx, d.ID, code, sendErr.Error())
	metrics.DeliveryFailedTotal.WithLabelValues(device.AppID, d.Provider).Inc()
	w.maybeCompleteMessage(ctx, msg.ID)
}

func (w *Worker) maybeCompleteMessage(ctx context.Context, messageID string) {
	pending, err := w.deliveries.HasPending(ctx, messageID)
	if err != nil || pending {
		return
	}
	total, terminal, err := w.deliveries.CountByMessage(ctx, messageID)
	if err != nil || total == 0 {
		return
	}
	if terminal < total {
		return
	}
	_ = w.messages.MarkCompleted(ctx, messageID)
}
