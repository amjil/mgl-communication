package huawei

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

func TestBuildRequestBody(t *testing.T) {
	b, err := buildRequestBody(&domain.Message{
		ID:       "01K",
		Title:    "Nomio",
		Body:     "hello",
		DeepLink: "nomio://a/1",
		Data:     map[string]string{"type": "comment"},
		Priority: domain.PriorityHigh,
		TTL:      60 * time.Second,
	}, "tok-1")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatal(err)
	}
	msg := root["message"].(map[string]any)
	tokens := msg["token"].([]any)
	if tokens[0] != "tok-1" {
		t.Fatal(tokens)
	}
	dataStr := msg["data"].(string)
	var data map[string]string
	_ = json.Unmarshal([]byte(dataStr), &data)
	if data["mgl_message_id"] != "01K" || data["type"] != "comment" {
		t.Fatalf("%v", data)
	}
	n := msg["notification"].(map[string]any)
	if n["title"] != "Nomio" {
		t.Fatal(n)
	}
}

func TestMapPushResponse(t *testing.T) {
	res := mapPushResponse(200, []byte(`{"code":"80000000","msg":"Success","requestId":"r1"}`))
	if !res.Accepted || res.ProviderMessageID != "r1" {
		t.Fatal(res)
	}
	res = mapPushResponse(200, []byte(`{"code":"80300007","msg":"All the tokens are invalid"}`))
	if !res.InvalidToken {
		t.Fatal(res)
	}
	res = mapPushResponse(200, []byte(`{"code":"80300010","msg":"too many"}`))
	if !res.Retryable || res.ErrorCode != domain.ErrCodeProviderRateLimit {
		t.Fatal(res)
	}
}

func TestSendWithMockServers(t *testing.T) {
	oauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Fatalf("grant=%s", r.Form.Get("grant_type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"at-1","expires_in":3600}`))
	}))
	defer oauth.Close()

	push := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer at-1") {
			t.Fatalf("auth=%s", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		var root map[string]any
		_ = json.Unmarshal(body, &root)
		_, _ = w.Write([]byte(`{"code":"80000000","msg":"Success","requestId":"req-9"}`))
	}))
	defer push.Close()

	p, err := New(Config{
		AppID:     "app",
		AppSecret: "sec",
		TokenURL:  oauth.URL,
		PushURL:   push.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := p.Send(t.Context(), &domain.Message{Title: "t", Body: "b", ID: "01"}, &domain.Device{Token: "devtok"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.ProviderMessageID != "req-9" {
		t.Fatalf("%+v", res)
	}
}
