package router

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/pterodactyl/wings/config"
	"github.com/pterodactyl/wings/internal/hoststats"
	"github.com/pterodactyl/wings/server"
)

func TestPostUpdateConfigurationRotatesCredentials(t *testing.T) {
	t.Setenv("WINGS_TOKEN_ID", "")
	t.Setenv("WINGS_TOKEN", "")

	cfg, err := config.NewAtPath(filepath.Join(t.TempDir(), "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.AuthenticationTokenId = "old-id"
	cfg.AuthenticationToken = "old-token"
	if err := cfg.ResolveToken(false); err != nil {
		t.Fatal(err)
	}
	config.Set(cfg)

	credentials := make(chan [2]string, 1)
	manager := server.NewEmptyManager(backupTestRemoteClient{credentials: credentials})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("manager", manager)
	c.Request = httptest.NewRequest("POST", "/api/update", strings.NewReader(`{"token_id":"new-id","token":"new-token"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	postUpdateConfiguration(c)

	if recorder.Code != 200 {
		t.Fatalf("expected successful update, got status %d", recorder.Code)
	}
	updated := config.Get()
	if updated.Token.ID != "new-id" || updated.Token.Token != "new-token" {
		t.Fatalf("unexpected resolved credentials: %#v", updated.Token)
	}
	select {
	case got := <-credentials:
		if got != [2]string{"new-id", "new-token"} {
			t.Fatalf("unexpected client credentials: %#v", got)
		}
	default:
		t.Fatal("expected client credentials to be rotated")
	}
}

func TestGetSystemUtilizationWithoutMonitor(t *testing.T) {
	hoststats.Set(nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/system/utilization", nil)

	getSystemUtilization(c)

	if recorder.Code != 503 {
		t.Fatalf("expected a 503 when the monitor is disabled, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "host monitor disabled") {
		t.Fatalf("expected the body to explain why, got %q", recorder.Body.String())
	}
}

func TestGetSystemUtilizationBeforeFirstSample(t *testing.T) {
	hoststats.Set(hoststats.New(config.HostMonitor{Enabled: true, Interval: 2}, func() []hoststats.ServerUsage { return nil }, ""))
	t.Cleanup(func() { hoststats.Set(nil) })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/system/utilization", nil)

	getSystemUtilization(c)

	if recorder.Code != 204 {
		t.Fatalf("expected a 204 before the first sample, got %d", recorder.Code)
	}
}

func TestGetSystemUtilizationReturnsSnapshot(t *testing.T) {
	sampler := hoststats.New(config.HostMonitor{Enabled: true, Interval: 2}, func() []hoststats.ServerUsage { return nil }, "")
	sampler.Collect()
	hoststats.Set(sampler)
	t.Cleanup(func() { hoststats.Set(nil) })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/system/utilization", nil)

	getSystemUtilization(c)

	if recorder.Code != 200 {
		t.Fatalf("expected a 200 once a sample exists, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"pressure"`) {
		t.Fatalf("expected a snapshot in the body, got %q", recorder.Body.String())
	}
}
