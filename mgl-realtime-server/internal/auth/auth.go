package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
)

type contextKey string

const (
	appIDKey    contextKey = "app_id"
	userIDKey   contextKey = "user_id"
	deviceIDKey contextKey = "device_id"
	authKindKey contextKey = "auth_kind"
)

const (
	AuthKindService = "service"
	AuthKindUser    = "user"
)

type Authenticator struct {
	tokens map[string]string // bearer token -> app_id
	jwt    *JWTValidator
}

func New(tokens map[string]string) *Authenticator {
	return &Authenticator{tokens: tokens}
}

func (a *Authenticator) WithJWT(v *JWTValidator) *Authenticator {
	a.jwt = v
	return a
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/health" || path == "/ready" || path == "/metrics" || path == "/ws" {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeUnauthorized(w)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

		if appID, ok := a.tokens[token]; ok && appID != "" {
			ctx := context.WithValue(r.Context(), appIDKey, appID)
			ctx = context.WithValue(ctx, authKindKey, AuthKindService)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if a.jwt != nil {
			claims, err := a.jwt.Validate(token)
			if err == nil {
				appID := claims.App
				if appID == "" {
					writeUnauthorized(w)
					return
				}
				ctx := context.WithValue(r.Context(), appIDKey, appID)
				ctx = context.WithValue(ctx, userIDKey, claims.Subject)
				ctx = context.WithValue(ctx, deviceIDKey, claims.DeviceID)
				ctx = context.WithValue(ctx, authKindKey, AuthKindUser)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		writeUnauthorized(w)
	})
}

func AppIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(appIDKey).(string)
	return v
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func DeviceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(deviceIDKey).(string)
	return v
}

func AuthKindFromContext(ctx context.Context) string {
	v, _ := ctx.Value(authKindKey).(string)
	return v
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"Unauthorized"}}`))
}

func RequireAppID(ctx context.Context) (string, error) {
	appID := AppIDFromContext(ctx)
	if appID == "" {
		return "", domain.Unauthorized("missing app context")
	}
	return appID, nil
}

func RequireUserID(ctx context.Context, bodyUserID string) (string, error) {
	if uid := UserIDFromContext(ctx); uid != "" {
		return uid, nil
	}
	if bodyUserID != "" {
		return bodyUserID, nil
	}
	return "", domain.Unauthorized("missing user_id")
}
