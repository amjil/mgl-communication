package xiaomi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amjil/mgl-push/mgl-push-server/internal/domain"
)

func TestBuildFormNotification(t *testing.T) {
	form, err := buildForm(&domain.Message{
		ID:       "01K",
		Title:    "Nomio",
		Body:     "hello",
		DeepLink: "nomio://a/1",
		Data:     map[string]string{"type": "comment"},
		TTL:      60 * time.Second,
	}, &domain.Device{Token: "reg-1", AppID: "net.amjil.demo"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("registration_id") != "reg-1" {
		t.Fatal(form)
	}
	if form.Get("restricted_package_name") != "net.amjil.demo" {
		t.Fatal(form)
	}
	if form.Get("pass_through") != "0" {
		t.Fatal("expected notification")
	}
	if !strings.Contains(form.Get("payload"), "mgl_message_id") {
		t.Fatal(form.Get("payload"))
	}
	if form.Get("time_to_live") != "60000" {
		t.Fatalf("ttl=%s", form.Get("time_to_live"))
	}
}

func TestBuildFormDataOnly(t *testing.T) {
	form, err := buildForm(&domain.Message{
		Data: map[string]string{"type": "sync"},
	}, &domain.Device{Token: "r", AppID: "pkg"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("pass_through") != "1" {
		t.Fatal(form)
	}
}

func TestMapResponse(t *testing.T) {
	res := mapResponse(200, []byte(`{"result":"ok","code":0,"data":{"id":"m1"}}`))
	if !res.Accepted || res.ProviderMessageID != "m1" {
		t.Fatal(res)
	}
	res = mapResponse(200, []byte(`{"result":"error","code":20301,"reason":"Invalid registration id"}`))
	if !res.InvalidToken {
		t.Fatal(res)
	}
	res = mapResponse(200, []byte(`{"result":"error","code":10008,"reason":"Invalid secret"}`))
	if res.ErrorCode != domain.ErrCodeProviderAuthError {
		t.Fatal(res)
	}
}

func TestSendMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "key=secret" {
			t.Fatalf("auth=%s", r.Header.Get("Authorization"))
		}
		_ = r.ParseForm()
		if r.Form.Get("registration_id") != "tok" {
			t.Fatal(r.Form)
		}
		_, _ = io.WriteString(w, `{"result":"ok","code":0,"data":{"id":"xid"}}`)
	}))
	defer srv.Close()

	p, err := New(Config{AppSecret: "secret", SendURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Send(t.Context(), &domain.Message{Title: "t", Body: "b"}, &domain.Device{
		Token: "tok", AppID: "net.amjil.demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Accepted || res.ProviderMessageID != "xid" {
		t.Fatalf("%+v", res)
	}
}
