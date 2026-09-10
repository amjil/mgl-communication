package oppo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

func TestBuildMessageJSON(t *testing.T) {
	b, err := buildMessageJSON(&domain.Message{
		ID:       "01K",
		Title:    "Nomio",
		Body:     "hello",
		DeepLink: "nomio://a/1",
		Data:     map[string]string{"type": "comment"},
		Category: "mgl_default",
	}, "reg-1")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if int(m["target_type"].(float64)) != 2 {
		t.Fatal(m["target_type"])
	}
	if m["target_value"] != "reg-1" {
		t.Fatal(m["target_value"])
	}
	n := m["notification"].(map[string]any)
	if n["title"] != "Nomio" || n["channel_id"] != "mgl_default" {
		t.Fatal(n)
	}
	if !strings.Contains(n["action_parameters"].(string), "mgl_message_id") {
		t.Fatal(n["action_parameters"])
	}
}

func TestMapResponse(t *testing.T) {
	res := mapResponse([]byte(`{"code":0,"message":"success","data":{"messageId":"m1"}}`))
	if !res.Accepted || res.ProviderMessageID != "m1" {
		t.Fatal(res)
	}
	res = mapResponse([]byte(`{"code":11,"message":"Invalid registration_id"}`))
	if !res.InvalidToken {
		t.Fatal(res)
	}
}

func TestAuthSign(t *testing.T) {
	ts := "1700000000000"
	want := sha256.Sum256([]byte("key" + ts + "secret"))
	got := sha256Hex("key" + ts + "secret")
	if got != hex.EncodeToString(want[:]) {
		t.Fatal(got)
	}
}

func TestSendWithMock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("app_key") != "ak" {
			t.Fatal(r.Form)
		}
		if r.Form.Get("sign") == "" {
			t.Fatal("missing sign")
		}
		_, _ = io.WriteString(w, `{"code":0,"message":"success","data":{"auth_token":"tok","create_time":1}}`)
	})
	mux.HandleFunc("/server/v1/message/notification/unicast", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("auth_token") != "tok" {
			t.Fatal(r.Form)
		}
		var msg map[string]any
		_ = json.Unmarshal([]byte(r.Form.Get("message")), &msg)
		if msg["target_value"] != "rid" {
			t.Fatal(msg)
		}
		_, _ = io.WriteString(w, `{"code":0,"message":"success","data":{"messageId":"mid-9"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, err := New(Config{AppKey: "ak", MasterSecret: "ms", Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Send(t.Context(), &domain.Message{Title: "t", Body: "b"}, &domain.Device{Token: "rid"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.ProviderMessageID != "mid-9" {
		t.Fatalf("%+v", res)
	}
}
