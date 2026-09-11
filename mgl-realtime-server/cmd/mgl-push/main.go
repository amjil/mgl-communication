package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/api"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/call"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/config"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/events"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/incomingcall"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/phoenix"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/presence"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider"
	apnspkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/apns"
	fcmpkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/fcm"
	huaweipkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/huawei"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/noop"
	oppopkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/oppo"
	vivopkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/vivo"
	xiaomipkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/provider/xiaomi"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/queue"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/repository"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/service"
	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/sfu"
	livekitpkg "github.com/amjil/mgl-communication/mgl-realtime-server/internal/sfu/livekit"
	wshub "github.com/amjil/mgl-communication/mgl-realtime-server/internal/signaling/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	deviceRepo := repository.NewDeviceRepository(pool)
	messageRepo := repository.NewMessageRepository(pool)
	deliveryRepo := repository.NewDeliveryRepository(pool)

	deviceSvc := service.NewDeviceService(deviceRepo)
	messageSvc := service.NewMessageService(messageRepo, deliveryRepo, deviceRepo)

	registry := provider.NewRegistry()
	registerProviders(ctx, cfg, registry, logger)

	worker := queue.NewWorker(
		messageRepo, deliveryRepo, deviceRepo, registry,
		cfg.WorkerPollInterval, cfg.WorkerBatchSize, cfg.MaxAttempts, logger,
	)
	worker.Start(ctx)

	bus := events.NewBus(512)
	jwtValidator := auth.NewJWTValidator(cfg.JWTSecret, cfg.JWTIssuer)

	// 1. 初始化 Redis Client 及分布式总线
	var redisBus *events.RedisBus
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Error("redis url parse failed", "error", err)
			os.Exit(1)
		}
		redisClient = redis.NewClient(opt)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Error("redis ping failed", "error", err)
			os.Exit(1)
		}
		redisBus = events.NewRedisBus(redisClient, logger)
		logger.Info("redis pub/sub enabled", "url", cfg.RedisURL)
	} else {
		logger.Warn("MGL_PUSH_REDIS_URL unset; WebSocket delivery is process-local only")
	}

	var sfuProvider sfu.Provider = sfu.NewStub(cfg.LiveKitURL)
	if cfg.LiveKitAPIKey != "" && cfg.LiveKitAPISecret != "" {
		sfuProvider = livekitpkg.New(cfg.LiveKitURL, cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)
		logger.Info("livekit sfu configured", "url", cfg.LiveKitURL)
	} else {
		logger.Warn("LiveKit credentials unset; SFU uses stub provider")
	}
	sfuSvc := sfu.NewService(sfuProvider, cfg.ICEServers)

	// 2. 初始化 Presence 与 Call Store
	var presenceStore presence.Store
	var callStore call.Store

	if redisClient != nil {
		presenceStore = presence.NewRedisStore(redisClient, bus, cfg.PresenceIdleAfter)
		callStore = call.NewRedisStore(redisClient)
		logger.Info("runtime state store: redis (cluster mode enabled)")
	} else {
		presenceStore = presence.NewMemoryStore(bus)
		callStore = call.NewMemoryStore()
		logger.Warn("runtime state store: memory (process-local only)")
	}

	callRuntime := call.NewService(callStore, bus, cfg.CallRingTimeout)
	orchestrator := &call.Orchestrator{
		Runtime:    callRuntime,
		Authz:      phoenix.NewFromConfig(cfg.PhoenixBaseURL, cfg.PhoenixToken, cfg.PhoenixTimeout),
		SFU:        sfuSvc,
		JWT:        jwtValidator,
		TokenTTL:   cfg.CallTokenTTL,
		ICEServers: cfg.ICEServers,
	}

	incomingSvc := incomingcall.New(messageSvc, bus, logger)
	callRuntime.SetRingTimeoutHandler(func(callID string) {
		c, err := callRuntime.Get(context.Background(), callID)
		if err != nil {
			return
		}
		incomingSvc.NotifyCancelled(context.Background(), c, "ring_timeout")
	})

	hub := wshub.NewHub(
		jwtValidator,
		presenceStore,
		orchestrator,
		incomingSvc,
		bus,
		redisBus,
		logger,
		wshub.HubConfig{
			PingInterval:   cfg.WSPingInterval,
			ReadTimeout:    cfg.WSReadTimeout,
			MsgPerSec:      cfg.WSMsgPerSec,
			CallsPerMinute: cfg.CallsPerMinute,
		},
	)

	apiServer := api.NewServer(api.Deps{
		Devices:  deviceSvc,
		Messages: messageSvc,
		Calls:    orchestrator,
		Presence: presenceStore,
		Incoming: incomingSvc,
		Hub:      hub,
		DB:       pool,
		Registry: registry,
		Worker:   worker,
	})
	authenticator := auth.New(cfg.ServiceTokens).WithJWT(jwtValidator)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           apiServer.Handler(authenticator),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("http listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownCtx)
	worker.Stop()
	_ = registry.Close()
	if redisClient != nil {
		_ = redisClient.Close()
	}
	pool.Close()
	logger.Info("bye")
}

func registerProviders(ctx context.Context, cfg *config.Config, registry *provider.Registry, logger *slog.Logger) {
	if cfg.HuaweiAppID != "" && cfg.HuaweiAppSecret != "" {
		p, err := huaweipkg.New(huaweipkg.Config{
			AppID:     cfg.HuaweiAppID,
			AppSecret: cfg.HuaweiAppSecret,
		})
		if err != nil {
			logger.Error("huawei provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderHuawei))
		} else {
			registry.Register(p)
			logger.Info("huawei provider registered")
		}
	} else {
		registry.Register(noop.New(domain.ProviderHuawei))
		logger.Warn("Huawei credentials incomplete; huawei uses noop")
	}

	if cfg.XiaomiAppSecret != "" {
		p, err := xiaomipkg.New(xiaomipkg.Config{
			AppSecret:   cfg.XiaomiAppSecret,
			PackageName: cfg.XiaomiPackageName,
		})
		if err != nil {
			logger.Error("xiaomi provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderXiaomi))
		} else {
			registry.Register(p)
			logger.Info("xiaomi provider registered")
		}
	} else {
		registry.Register(noop.New(domain.ProviderXiaomi))
		logger.Warn("Xiaomi credentials incomplete; xiaomi uses noop")
	}

	if cfg.OppoAppKey != "" && cfg.OppoMasterSecret != "" {
		p, err := oppopkg.New(oppopkg.Config{
			AppKey:       cfg.OppoAppKey,
			MasterSecret: cfg.OppoMasterSecret,
		})
		if err != nil {
			logger.Error("oppo provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderOppo))
		} else {
			registry.Register(p)
			logger.Info("oppo provider registered")
		}
	} else {
		registry.Register(noop.New(domain.ProviderOppo))
		logger.Warn("OPPO credentials incomplete; oppo uses noop")
	}

	if cfg.VivoAppID != "" && cfg.VivoAppKey != "" && cfg.VivoAppSecret != "" {
		p, err := vivopkg.New(vivopkg.Config{
			AppID:     cfg.VivoAppID,
			AppKey:    cfg.VivoAppKey,
			AppSecret: cfg.VivoAppSecret,
		})
		if err != nil {
			logger.Error("vivo provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderVivo))
		} else {
			registry.Register(p)
			logger.Info("vivo provider registered")
		}
	} else {
		registry.Register(noop.New(domain.ProviderVivo))
		logger.Warn("vivo credentials incomplete; vivo uses noop")
	}

	if cfg.FCMCredentialsFile != "" {
		p, err := fcmpkg.NewFromCredentialsFile(ctx, cfg.FCMCredentialsFile)
		if err != nil {
			logger.Error("fcm provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderFCM))
		} else {
			registry.Register(p)
			logger.Info("fcm provider registered")
		}
	} else {
		registry.Register(noop.New(domain.ProviderFCM))
		logger.Warn("MGL_PUSH_FCM_CREDENTIALS_FILE unset; fcm uses noop")
	}

	if cfg.APNsTeamID != "" && cfg.APNsKeyID != "" && cfg.APNsPrivateKey != "" && cfg.APNsBundleID != "" {
		p, err := apnspkg.New(apnspkg.Config{
			TeamID:     cfg.APNsTeamID,
			KeyID:      cfg.APNsKeyID,
			BundleID:   cfg.APNsBundleID,
			PrivateKey: cfg.APNsPrivateKey,
			Production: cfg.APNsProduction,
		})
		if err != nil {
			logger.Error("apns provider init failed; falling back to noop", "error", err)
			registry.Register(noop.New(domain.ProviderAPNs))
			registry.Register(noop.New(domain.ProviderAPNsVoIP))
		} else {
			registry.Register(p)
			registry.Register(provider.NewNamed(domain.ProviderAPNsVoIP, p))
			logger.Info("apns provider registered", "production", cfg.APNsProduction)
		}
	} else {
		registry.Register(noop.New(domain.ProviderAPNs))
		registry.Register(noop.New(domain.ProviderAPNsVoIP))
		logger.Warn("APNs credentials incomplete; apns uses noop")
	}

	logger.Info("providers registered", "names", registry.Names(), "env", cfg.Env)
}
