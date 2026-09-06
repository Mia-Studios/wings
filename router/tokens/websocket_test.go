package tokens

import (
	"testing"
	"time"

	"github.com/gbrlsnchs/jwt/v3"
)

func testPayload(permissions ...string) *WebsocketPayload {
	issued := jwt.NumericDate(time.Now().Add(time.Minute))

	return &WebsocketPayload{
		Payload:     jwt.Payload{JWTID: "test", IssuedAt: issued},
		UserUUID:    "9f7dcd8e-9b2d-4d1a-9a06-0f0d0a2a1234",
		ServerUUID:  "1a3f5d76-5f6e-4c7b-8d9a-0b1c2d3e4f50",
		Permissions: permissions,
	}
}

func TestHasPermissionGrantsExactMatch(t *testing.T) {
	p := testPayload("websocket.connect", "admin.websocket.host")

	if !p.HasPermission("admin.websocket.host") {
		t.Fatal("expected an explicitly granted admin permission to be accepted")
	}
}

func TestHasPermissionWildcardDoesNotGrantAdminPermissions(t *testing.T) {
	p := testPayload("*")

	if p.HasPermission("admin.websocket.host") {
		t.Fatal("expected the wildcard permission to not cover admin permissions")
	}
	if !p.HasPermission("websocket.connect") {
		t.Fatal("expected the wildcard permission to still cover regular permissions")
	}
}

func TestHasPermissionRejectsMissingPermission(t *testing.T) {
	p := testPayload("websocket.connect", "admin.websocket.errors")

	if p.HasPermission("admin.websocket.host") {
		t.Fatal("expected a payload without the host permission to be rejected")
	}
}
