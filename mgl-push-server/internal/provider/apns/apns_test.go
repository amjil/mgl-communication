package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

func TestBuildPayloadAlert(t *testing.T) {
	badge := 3
	b, err := buildPayload(&domain.Message{
		ID:       "01KTEST",
		Title:    "Nomio",
		Body:     "hello",
		Badge:    &badge,
		DeepLink: "nomio://a/1",
		Data:     map[string]string{"type": "comment"},
		Sound:    "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	aps := m["aps"].(map[string]any)
	alert := aps["alert"].(map[string]any)
	if alert["title"] != "Nomio" || alert["body"] != "hello" {
		t.Fatalf("%v", alert)
	}
	if m["mgl_message_id"] != "01KTEST" {
		t.Fatal("mgl_message_id")
	}
	if m["deep_link"] != "nomio://a/1" {
		t.Fatal("deep_link")
	}
	if m["type"] != "comment" {
		t.Fatal("custom data")
	}
}

func TestBuildPayloadDataOnly(t *testing.T) {
	b, err := buildPayload(&domain.Message{
		ID:   "id",
		Data: map[string]string{"type": "sync"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	aps := m["aps"].(map[string]any)
	if aps["content-available"].(float64) != 1 {
		t.Fatalf("%v", aps)
	}
}

func TestMapAPNsError(t *testing.T) {
	res := mapAPNsError(410, []byte(`{"reason":"Unregistered"}`))
	if !res.InvalidToken {
		t.Fatal(res)
	}
	res = mapAPNsError(429, []byte(`{"reason":"TooManyRequests"}`))
	if !res.Retryable || res.ErrorCode != domain.ErrCodeProviderRateLimit {
		t.Fatal(res)
	}
	res = mapAPNsError(403, []byte(`{"reason":"InvalidProviderToken"}`))
	if res.ErrorCode != domain.ErrCodeProviderAuthError {
		t.Fatal(res)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSendAcceptedWithMockTransport(t *testing.T) {
	_, pemBytes := mustTestKey(t)

	p, err := New(Config{
		TeamID:     "TEAM123",
		KeyID:      "KEY123",
		BundleID:   "net.amjil.demo",
		PrivateKey: string(pemBytes),
		Production: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	p.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("authorization") == "" {
				t.Error("missing auth")
			}
			if r.Header.Get("apns-topic") != "net.amjil.demo" {
				t.Errorf("topic=%s", r.Header.Get("apns-topic"))
			}
			if r.Header.Get("apns-push-type") != "alert" {
				t.Errorf("push-type=%s", r.Header.Get("apns-push-type"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Apns-Id": []string{"mock-apns-id"}},
				Body:       io.NopCloser(http.NoBody),
				Request:    r,
			}, nil
		}),
	}

	res, err := p.Send(t.Context(), &domain.Message{
		ID:    "01K",
		Title: "t",
		Body:  "b",
	}, &domain.Device{Token: "aabbccddeeff00112233445566778899"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.ProviderMessageID != "mock-apns-id" {
		t.Fatalf("%+v", res)
	}
}

func TestSendInvalidTokenResponse(t *testing.T) {
	_, pemBytes := mustTestKey(t)
	p, err := New(Config{
		TeamID: "T", KeyID: "K", BundleID: "b",
		PrivateKey: string(pemBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	p.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader(`{"reason":"BadDeviceToken"}`)),
				Request:    r,
			}, nil
		}),
	}
	res, err := p.Send(t.Context(), &domain.Message{Title: "t"}, &domain.Device{Token: "badtokenbadtoken12"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.InvalidToken {
		t.Fatalf("%+v", res)
	}
}

func mustTestKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return key, pemBytes
}
