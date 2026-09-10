package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

type contextKey string

const appIDKey contextKey = "app_id"

type Authenticator struct {
	tokens map[string]string // bearer token -> app_id
}

func New(tokens map[string]string) *Authenticator {
	return &Authenticator{tokens: tokens}
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/health" || path == "/ready" || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeUnauthorized(w)
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		appID, ok := a.tokens[token]
		if !ok || appID == "" {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), appIDKey, appID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AppIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(appIDKey).(string)
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
