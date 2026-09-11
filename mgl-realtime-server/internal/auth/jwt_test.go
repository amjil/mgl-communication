package auth_test

import (
	"testing"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/auth"
)

func TestJWTRoundTrip(t *testing.T) {
	v := auth.NewJWTValidator("secret", "mgl-realtime")
	tok, err := v.IssueDevToken("user_1", "nomio", "device_1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := v.Validate(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user_1" || claims.App != "nomio" {
		t.Fatalf("claims=%+v", claims)
	}
}

func TestCallToken(t *testing.T) {
	v := auth.NewJWTValidator("secret", "mgl-realtime")
	tok, err := v.IssueCallToken("call_1", "user_1", "room_1", "publisher", "nomio", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := v.ValidateCallToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.CallID != "call_1" || claims.RoomID != "room_1" {
		t.Fatalf("claims=%+v", claims)
	}
}
