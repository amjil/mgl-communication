package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_messages_total",
		Help: "Total push messages created",
	}, []string{"app_id", "platform"})

	MessagesFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_messages_failed_total",
		Help: "Total push messages that failed",
	}, []string{"app_id", "provider"})

	MessagesAcceptedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_messages_accepted_total",
		Help: "Total push messages accepted by providers",
	}, []string{"app_id", "provider"})

	MessagesRetriedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_messages_retried_total",
		Help: "Total push delivery retries",
	}, []string{"app_id", "provider"})

	DeliveryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_delivery_total",
		Help: "Total push deliveries",
	}, []string{"app_id", "provider", "platform"})

	DeliveryFailedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_delivery_failed_total",
		Help: "Total failed deliveries",
	}, []string{"app_id", "provider"})

	ProviderLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "push_provider_latency",
		Help:    "Provider send latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider"})

	ProviderErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_provider_errors",
		Help: "Provider errors",
	}, []string{"provider", "code"})

	InvalidTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "push_invalid_tokens_total",
		Help: "Invalid tokens detected",
	}, []string{"provider", "app_id"})

	IncomingCallPushTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_call_push_total",
		Help: "Incoming call push messages created",
	}, []string{"app_id"})

	IncomingCallPushFailureTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "incoming_call_push_failure_total",
		Help: "Incoming call push failures",
	}, []string{"app_id", "provider"})
)
