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

func Load() *Config {
	cfg := &Config{
		Env:                      getenv("MGL_PUSH_ENV", "development"),
		HTTPAddr:                 getenv("MGL_PUSH_HTTP_ADDR", ":8080"),
		DatabaseURL:              getenv("MGL_PUSH_DATABASE_URL", "postgres://mglpush:mglpush@localhost:5432/mglpush?sslmode=disable"),
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
	}
	if len(cfg.ServiceTokens) == 0 {
		// Development default: token "dev-token" for app "net.amjil.demo"
		cfg.ServiceTokens = map[string]string{"dev-token": "net.amjil.demo"}
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
