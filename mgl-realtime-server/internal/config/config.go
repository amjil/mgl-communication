package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string

	ServiceTokens map[string]string // token -> app_id

	// User JWT for WebSocket / client APIs (HS256).
	JWTSecret string
	JWTIssuer string

	// Phoenix business authorization (empty = allow-all in development).
	PhoenixBaseURL string
	PhoenixToken   string
	PhoenixTimeout time.Duration

	// ICE / TURN for P2P calls (Go issues config; does not run TURN).
	ICEServers []ICEServer

	// LiveKit SFU (optional; group calls use stub when unset).
	LiveKitURL       string
	LiveKitAPIKey    string
	LiveKitAPISecret string

	CallRingTimeout    time.Duration
	CallTokenTTL       time.Duration
	WSPingInterval     time.Duration
	WSReadTimeout      time.Duration
	WSMsgPerSec        int
	CallsPerMinute     int
	PresenceIdleAfter  time.Duration

	RateLimitPerService      int
	RateLimitPerUser         int
	RateLimitPerInstallation int
	RateLimitWindow          time.Duration

	WorkerPollInterval time.Duration
	WorkerBatchSize    int
	MaxAttempts        int

	HuaweiAppID     string
	HuaweiAppSecret string

	XiaomiAppID       string
	XiaomiAppSecret   string
	XiaomiPackageName string

	OppoAppKey       string
	OppoMasterSecret string

	VivoAppID     string
	VivoAppKey    string
	VivoAppSecret string

	FCMCredentialsFile string

	APNsTeamID     string
	APNsKeyID      string
	APNsPrivateKey string
	APNsBundleID   string
	APNsProduction bool
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func Load() *Config {
	cfg := &Config{
		Env:                      getenv("MGL_PUSH_ENV", "development"),
		HTTPAddr:                 getenv("MGL_PUSH_HTTP_ADDR", ":8080"),
		DatabaseURL:              getenv("MGL_PUSH_DATABASE_URL", "postgres://mglpush:mglpush@localhost:5432/mglpush?sslmode=disable"),
		JWTSecret:                getenv("MGL_PUSH_JWT_SECRET", "dev-jwt-secret-change-me"),
		JWTIssuer:                getenv("MGL_PUSH_JWT_ISSUER", "mgl-realtime"),
		PhoenixBaseURL:           os.Getenv("MGL_PUSH_PHOENIX_BASE_URL"),
		PhoenixToken:             os.Getenv("MGL_PUSH_PHOENIX_TOKEN"),
		PhoenixTimeout:           time.Duration(getenvInt("MGL_PUSH_PHOENIX_TIMEOUT_MS", 3000)) * time.Millisecond,
		LiveKitURL:               os.Getenv("MGL_PUSH_LIVEKIT_URL"),
		LiveKitAPIKey:            os.Getenv("MGL_PUSH_LIVEKIT_API_KEY"),
		LiveKitAPISecret:         os.Getenv("MGL_PUSH_LIVEKIT_API_SECRET"),
		CallRingTimeout:          time.Duration(getenvInt("MGL_PUSH_CALL_RING_TIMEOUT_SEC", 45)) * time.Second,
		CallTokenTTL:             time.Duration(getenvInt("MGL_PUSH_CALL_TOKEN_TTL_SEC", 300)) * time.Second,
		WSPingInterval:           time.Duration(getenvInt("MGL_PUSH_WS_PING_SEC", 25)) * time.Second,
		WSReadTimeout:            time.Duration(getenvInt("MGL_PUSH_WS_READ_TIMEOUT_SEC", 60)) * time.Second,
		WSMsgPerSec:              getenvInt("MGL_PUSH_WS_MSG_PER_SEC", 30),
		CallsPerMinute:           getenvInt("MGL_PUSH_CALLS_PER_MINUTE", 20),
		PresenceIdleAfter:        time.Duration(getenvInt("MGL_PUSH_PRESENCE_IDLE_SEC", 300)) * time.Second,
		RateLimitPerService:      getenvInt("MGL_PUSH_RATE_LIMIT_SERVICE", 1000),
		RateLimitPerUser:         getenvInt("MGL_PUSH_RATE_LIMIT_USER", 60),
		RateLimitPerInstallation: getenvInt("MGL_PUSH_RATE_LIMIT_INSTALLATION", 30),
		RateLimitWindow:          time.Minute,
		WorkerPollInterval:       time.Duration(getenvInt("MGL_PUSH_WORKER_POLL_MS", 1000)) * time.Millisecond,
		WorkerBatchSize:          getenvInt("MGL_PUSH_WORKER_BATCH", 50),
		MaxAttempts:              getenvInt("MGL_PUSH_MAX_ATTEMPTS", 5),
		HuaweiAppID:              os.Getenv("MGL_PUSH_HUAWEI_APP_ID"),
		HuaweiAppSecret:          os.Getenv("MGL_PUSH_HUAWEI_APP_SECRET"),
		XiaomiAppID:              os.Getenv("MGL_PUSH_XIAOMI_APP_ID"),
		XiaomiAppSecret:          os.Getenv("MGL_PUSH_XIAOMI_APP_SECRET"),
		XiaomiPackageName:        os.Getenv("MGL_PUSH_XIAOMI_PACKAGE_NAME"),
		OppoAppKey:               os.Getenv("MGL_PUSH_OPPO_APP_KEY"),
		OppoMasterSecret:         os.Getenv("MGL_PUSH_OPPO_MASTER_SECRET"),
		VivoAppID:                os.Getenv("MGL_PUSH_VIVO_APP_ID"),
		VivoAppKey:               os.Getenv("MGL_PUSH_VIVO_APP_KEY"),
		VivoAppSecret:            os.Getenv("MGL_PUSH_VIVO_APP_SECRET"),
		FCMCredentialsFile:       os.Getenv("MGL_PUSH_FCM_CREDENTIALS_FILE"),
		APNsTeamID:               os.Getenv("MGL_PUSH_APNS_TEAM_ID"),
		APNsKeyID:                os.Getenv("MGL_PUSH_APNS_KEY_ID"),
		APNsPrivateKey:           os.Getenv("MGL_PUSH_APNS_PRIVATE_KEY"),
		APNsBundleID:             os.Getenv("MGL_PUSH_APNS_BUNDLE_ID"),
		APNsProduction:           getenvBool("MGL_PUSH_APNS_PRODUCTION", getenv("MGL_PUSH_ENV", "development") == "production"),
		ServiceTokens:            parseServiceTokens(os.Getenv("MGL_PUSH_SERVICE_TOKENS")),
		ICEServers:               parseICEServers(os.Getenv("MGL_PUSH_ICE_SERVERS")),
	}
	if len(cfg.ServiceTokens) == 0 {
		// Development default: token "dev-token" for app "net.amjil.demo"
		cfg.ServiceTokens = map[string]string{"dev-token": "net.amjil.demo"}
	}
	if len(cfg.ICEServers) == 0 {
		cfg.ICEServers = []ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}
	}
	return cfg
}

func (c *Config) IsDevelopment() bool {
	return c.Env == "development" || c.Env == "dev"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// parseServiceTokens parses "token1:app1,token2:app2"
func parseServiceTokens(s string) map[string]string {
	out := make(map[string]string)
	if s == "" {
		return out
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return out
}

// parseICEServers parses comma-separated STUN/TURN URLs.
// Optional credentials: "turn:host:3478|user|pass"
func parseICEServers(s string) []ICEServer {
	if s == "" {
		return nil
	}
	var out []ICEServer
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, "|")
		srv := ICEServer{URLs: []string{strings.TrimSpace(fields[0])}}
		if len(fields) >= 3 {
			srv.Username = strings.TrimSpace(fields[1])
			srv.Credential = strings.TrimSpace(fields[2])
		}
		out = append(out, srv)
	}
	return out
}
