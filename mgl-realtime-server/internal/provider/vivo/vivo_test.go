package vivo

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/domain"
)

func TestBuildSendBody(t *testing.T) {
	b, err := buildSendBody(&domain.Message{
		ID:       "01KTESTMSG00000000000000001",
		Title:    "Nomio",
		Body:     "hello",
		DeepLink: "nomio://a/1",
		Data:     map[string]string{"type": "comment"},
		TTL:      60 * time.Second,
	}, "regid12345678901234567", 0)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["regId"] != "regid12345678901234567" {
		t.Fatal(m["regId"])
	}
	if int(m["skipType"].(float64)) != 3 {
		t.Fatal(m["skipType"])
	}
	custom := m["clientCustomMap"].(map[string]any)
	if custom["mgl_message_id"] == nil {
		t.Fatal(custom)
	}
}

func TestMapResponse(t *testing.T) {
	res := mapResponse([]byte(`{"result":0,"desc":"ok","taskId":"t1"}`))
	if !res.Accepted || res.ProviderMessageID != "t1" {
		t.Fatal(res)
	}
	res = mapResponse([]byte(`{"result":10302,"desc":"regId 不合法"}`))
	if !res.InvalidToken {
		t.Fatal(res)
	}
	res = mapResponse([]byte(`{"result":10000,"desc":"auth failed"}`))
	if res.ErrorCode != domain.ErrCodeProviderAuthError {
		t.Fatal(res)
	}
}

func TestMD5Sign(t *testing.T) {
	src := fmt.Sprintf("%d%s%d%s", 10004, "key", int64(1501484120000), "secret")
	sum := md5.Sum([]byte(src))
	if md5Hex(src) != hex.EncodeToString(sum[:]) {
		t.Fatal("md5 mismatch")
	}
}

func TestSendWithMock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/message/auth", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["appKey"] != "akey" {
			t.Fatal(body)
		}
		_, _ = io.WriteString(w, `{"result":0,"desc":"ok","authToken":"atok"}`)
	})
	mux.HandleFunc("/message/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("authToken") != "atok" {
			t.Fatal(r.Header.Get("authToken"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["regId"] != "rid" {
			t.Fatal(body)
		}
		if !strings.Contains(fmt.Sprint(body["title"]), "t") {
			t.Fatal(body["title"])
		}
		_, _ = io.WriteString(w, `{"result":0,"desc":"ok","taskId":"task-9"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p, err := New(Config{AppID: "12345", AppKey: "akey", AppSecret: "asec", Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Send(t.Context(), &domain.Message{Title: "t", Body: "b", ID: "01KREQ"}, &domain.Device{Token: "rid"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.ProviderMessageID != "task-9" {
		t.Fatalf("%+v", res)
	}
}
